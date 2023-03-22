package httpadapter

import (
	"bufio"
	"io"
	"net"
	"sync"
	"time"

	"github.com/powerpuffpenguin/httpadapter/core"
	"github.com/powerpuffpenguin/httpadapter/internal/memory"
)

type serverTransport struct {
	server  *Server
	handler Handler
	sync.Mutex
	keys map[uint64]*dataChannel
	baseTransport
}

func newServerTransport(server *Server,
	c net.Conn,
	window uint32,
	handler Handler,
) *serverTransport {
	return &serverTransport{
		server:  server,
		handler: handler,
		keys:    make(map[uint64]*dataChannel),
		baseTransport: baseTransport{
			allocator: server.opts.allocator,
			done:      make(chan struct{}),
			window:    window,
			c:         c,
			ch:        make(chan memory.Buffer, 50),
		},
	}
}

func (t *serverTransport) Serve(buffer memory.Buffer) {
	defer func() {
		t.Close()
		t.server.opts.allocator.Put(buffer)
	}()

	var (
		r          io.Reader = t.c
		e          error
		localAddr  = t.c.LocalAddr()
		remoteAddr = t.c.RemoteAddr()
		active     chan int
	)
	// 建立讀取緩存
	if t.server.opts.readBuffer > 0 {
		r = bufio.NewReaderSize(r, t.server.opts.readBuffer)
	}

	// ping
	if t.server.opts.ping > time.Second {
		active = make(chan int, 1)
		go t.servePing(active, t.server.opts.ping)
	}

	// 寫入 tcp-chain
	go t.serveWrite(active, t.server.opts.writeBuffer)

	b := buffer.Data
	// 讀取 tcp-chain
TS:
	for {
		if active != nil {
			select {
			case active <- 1:
			default:
			}
		}

		// 讀取指令
		_, e = io.ReadFull(r, b[:1])
		if e != nil {
			break
		}
		cmd := core.Command(b[0])
		switch cmd {
		case core.CommandPing:
		case core.CommandPong: // 響應 pong
			if t.onPong(r, b) {
				break TS
			}
		case core.CommandCreate: // 創建 channel
			_, e = io.ReadFull(r, b[:8])
			if e != nil {
				break TS
			}
			id := core.ByteOrder.Uint64(b)

			data := t.server.opts.allocator.Get(1 + 8 + 1)
			data.Data[0] = byte(core.CommandCreate)
			core.ByteOrder.PutUint64(data.Data[1:], id)

			t.Lock()
			_, exists := t.keys[id]
			if exists {
				data.Data[1+8] = 1
			} else if t.server.opts.channels > 0 && len(t.keys) >= t.server.opts.channels {
				data.Data[1+8] = 2
			} else {
				val := newChannel(t, id,
					localAddr, remoteAddr,
					int(t.server.opts.window), int(t.window),
				)
				go val.Serve()
				go t.handler.ServeChannel(t.server, val)
				t.keys[id] = val
				data.Data[1+8] = 0
			}
			t.Unlock()
			if t.postWrite(data) {
				break TS
			}
		case core.CommandClose: // 客戶端要求關閉 channel
			_, e = io.ReadFull(r, b[:8])
			if e != nil {
				break TS
			}
			id := core.ByteOrder.Uint64(b)
			t.Lock()
			if sc, exists := t.keys[id]; exists {
				sc.Close()
				delete(t.keys, id)
			}
			t.Unlock()
		case core.CommandWrite: // 向 channel 寫入數據
			_, e = io.ReadFull(r, b[:8+2])
			if e != nil {
				break TS
			}
			id := core.ByteOrder.Uint64(b)
			size := int(core.ByteOrder.Uint16(b[8:]))
			var data []byte
			if len(b) < size {
				data = make([]byte, size)
			} else {
				data = b[:size]
			}
			_, e = io.ReadFull(r, data)
			if e != nil {
				break TS
			}
			t.Lock()
			sc, exists := t.keys[id]
			t.Unlock()
			if exists {
				sc.Pipe(data)
			} else {
				t.sendClose(id)
				Logger.Printf(core.CommandWrite.String()+": channel(%v) not found\n", id)
			}
		case core.CommandConfirm: // 確認 channel 數據
			_, e = io.ReadFull(r, b[:8+2])
			if e != nil {
				break TS
			}
			id := core.ByteOrder.Uint64(b)
			t.Lock()
			sc, exists := t.keys[id]
			t.Unlock()
			if exists {
				if sc.Confirm(int(core.ByteOrder.Uint16(b[8:]))) {
					t.sendClose(id)
					Logger.Printf(core.CommandConfirm.String()+": channel(%v) overflow\n", id)
				}
			} else {
				t.sendClose(id)
				// Logger.Printf(core.CommandConfirm.String()+": channel(%v) not found\n", id)
			}
		default:
			Logger.Println(`Unknow Command:`, cmd.String())
			break TS
		}
	}
	// 清理 channel
	t.Lock()
	for _, c := range t.keys {
		c.Close()
	}
	t.Unlock()
}
func (t *serverTransport) getWriter() chan<- []byte {
	return t.ch
}
func (t *serverTransport) Done() <-chan struct{} {
	return t.done
}
func (t *serverTransport) delete(c *dataChannel) {
	deleted := false
	t.Lock()
	if t.keys[c.id] == c {
		delete(t.keys, c.id)
		deleted = true
	}
	t.Unlock()
	if !deleted {
		return
	}
	t.sendClose(c.id)
}
func (t *serverTransport) sendClose(id uint64) {
	b := make([]byte, 1+8)
	b[0] = byte(core.CommandClose)
	core.ByteOrder.PutUint64(b[1:], id)
	select {
	case <-t.done:
	case t.ch <- b:
	}
}
