package httpadapter

import (
	"bufio"
	"io"
	"net"
	"sync"
	"time"

	"github.com/powerpuffpenguin/httpadapter/core"
)

type serverTransport struct {
	server *Server

	keys map[uint64]*ioChannel
	sync.Mutex
	baseTransport
}

func newServerTransport(server *Server,
	c net.Conn,
	remoteWindow uint32,
) *serverTransport {
	return &serverTransport{
		server: server,
		keys:   make(map[uint64]*ioChannel),
		baseTransport: baseTransport{
			done:   make(chan struct{}),
			window: remoteWindow,
			c:      c,
			ch:     make(chan []byte, 50),
		},
	}
}

func (t *serverTransport) Serve(b []byte) {
	defer t.Close()

	var (
		opts                 = &t.server.opts
		r          io.Reader = t.c
		e          error
		localAddr  = t.c.LocalAddr()
		remoteAddr = t.c.RemoteAddr()
		active     chan int
	)
	// 建立讀取緩存
	if opts.readBuffer > 0 {
		r = bufio.NewReaderSize(r, opts.readBuffer)
	}

	// ping
	if opts.ping > time.Second {
		active = make(chan int, 1)
		go t.servePing(active, opts.ping)
	}

	// 寫入 tcp-chain
	go t.serveWrite(active, opts.writeBuffer)

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

			data := make([]byte, 1+8+1)
			data[0] = byte(core.CommandCreate)
			core.ByteOrder.PutUint64(data[1:], id)

			t.Lock()
			_, exists := t.keys[id]
			if exists {
				data[1+8] = 1
			} else if opts.channels > 0 && len(t.keys) >= opts.channels {
				data[1+8] = 2
			} else {
				val := newIOChannel(t, id,
					localAddr, remoteAddr,
					int(opts.window), int(t.window),
				)
				go val.Serve()
				go opts.channelHandler.ServeChannel(t.server, val)
				t.keys[id] = val
				data[1+8] = 0
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
			var buffer []byte
			if size < cap(b) {
				buffer = b[:size]
			} else if size > 32*1024 { // 避免申請 32k 以上的大內存
				buffer = make([]byte, 32*1024)
			} else {
				buffer = make([]byte, size)
			}
			var (
				n       int
				max     = len(buffer)
				healthy bool
			)
			for size != 0 {
				if size > max {
					n = max
				} else {
					n = size
				}
				_, e = io.ReadFull(r, buffer[:n])
				if e != nil {
					break TS
				}
				size -= n
				t.Lock()
				val, exists := t.keys[id]
				t.Unlock()
				if exists {
					val.Pipe(buffer[:n])
				} else {
					if healthy {
						healthy = false
						t.sendClose(id)
						Logger.Printf(core.CommandWrite.String()+": channel(%v) not found\n", id)
					}
				}
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
				if sc.Confirm(uint64(core.ByteOrder.Uint16(b[8:]))) {
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
func (t *serverTransport) delete(c *ioChannel) {
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
