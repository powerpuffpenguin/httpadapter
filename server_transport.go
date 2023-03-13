package httpadapter

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/powerpuffpenguin/httpadapter/core"
)

type serverTransport struct {
	c                    net.Conn
	window, remoteWindow int
	handler              Handler
	done                 chan struct{}
	closed               int32
	sync.Mutex
	keys map[uint64]*serverChannel
	ch   chan []byte
}

func newServerTransport(c net.Conn,
	window, remoteWindow int,
	handler Handler,
) *serverTransport {
	return &serverTransport{
		c:            c,
		window:       window,
		remoteWindow: remoteWindow,
		handler:      handler,
		done:         make(chan struct{}),
		keys:         make(map[uint64]*serverChannel),
		ch:           make(chan []byte, 100),
	}
}
func (t *serverTransport) Close() {
	if t.closed == 0 && atomic.SwapInt32(&t.closed, 1) == 0 {
		close(t.done)
		t.c.Close()
	}
}
func (t *serverTransport) Serve(b []byte,
	readBuffer, writeBuffer,
	channels int,
	ping time.Duration,
) {
	defer t.Close()

	var (
		r          io.Reader = t.c
		e          error
		ch         = t.ch
		localAddr  = t.c.LocalAddr()
		remoteAddr = t.c.RemoteAddr()
		active     chan int
	)
	if readBuffer > 0 {
		r = bufio.NewReaderSize(r, readBuffer)
	}

	// ping
	if ping > time.Second {
		active = make(chan int, 1)
		go core.Ping(
			ping,
			t.done, active,
			ch,
			[]byte{byte(core.CommandPing)},
		)
	}

	// 寫入 tcp-chain
	go core.Write(t.c, writeBuffer,
		t.done, active,
		ch,
	)

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
			if onPong(r, b, t.done, ch) {
				break TS
			}
		case core.CommandCreate: // 創建 channel
			_, e = io.ReadFull(r, b[:8])
			if e != nil {
				break TS
			}
			id := binary.BigEndian.Uint64(b)

			data := make([]byte, 1+8+1)
			data[0] = byte(core.CommandCreate)
			binary.BigEndian.PutUint64(data[1:], id)

			t.Lock()
			_, exists := t.keys[id]
			if exists {
				data[1+8] = 1
			} else if channels > 0 && len(t.keys) >= channels {
				data[1+8] = 2
			} else {
				val := newServerChannel(t, id,
					localAddr, remoteAddr,
					t.window, t.remoteWindow,
				)
				go val.Serve()
				go t.handler.ServeChannel(val)
				t.keys[id] = val
				data[1+8] = 0
			}
			t.Unlock()
			if core.PostWrite(t.done, t.ch, data) {
				break TS
			}
		case core.CommandClose: // 客戶端要求關閉 channel
			_, e = io.ReadFull(r, b[:8])
			if e != nil {
				break TS
			}
			id := binary.BigEndian.Uint64(b)
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
			id := binary.BigEndian.Uint64(b)
			size := int(binary.BigEndian.Uint16(b[8:]))
			data := make([]byte, size)
			_, e = io.ReadFull(r, b)
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
				ServerLogger.Printf(core.CommandWrite.String()+": channel(%v) not found\n", id)
			}
		case core.CommandConfirm: // 確認 channel 數據
			_, e = io.ReadFull(r, b[:8+2])
			if e != nil {
				break TS
			}
			id := binary.BigEndian.Uint64(b)
			t.Lock()
			sc, exists := t.keys[id]
			t.Unlock()
			if exists {
				if sc.Confirm(int(binary.BigEndian.Uint16(b[8:]))) {
					t.sendClose(id)
					ServerLogger.Printf(core.CommandConfirm.String()+": channel(%v) overflow\n", id)
				}
			} else {
				t.sendClose(id)
				ServerLogger.Printf(core.CommandConfirm.String()+": channel(%v) not found\n", id)
			}
		default:
			ServerLogger.Println(`Unknow Command:`, cmd.String())
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

func (t *serverTransport) delete(c *serverChannel) {
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
	binary.BigEndian.PutUint64(b[1:], id)
	select {
	case <-t.done:
	case t.ch <- b:
	}
}
