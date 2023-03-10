package httpadapter

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"sync/atomic"
)

type serverTransport struct {
	c                    net.Conn
	window, remoteWindow int
	done                 chan struct{}
	closed               int32
	sync.Mutex
	keys map[uint64]*serverChannel
	ch   chan []byte
}

func newServerTransport(c net.Conn,
	window, remoteWindow int,
) *serverTransport {
	return &serverTransport{
		c:            c,
		window:       window,
		remoteWindow: remoteWindow,
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
func (t *serverTransport) Serve(b []byte, readBuffer, writeBuffer, channels int) {
	defer t.Close()

	var (
		r          io.Reader = t.c
		e          error
		keys       = make(map[uint64]*serverChannel)
		ch         = t.ch
		localAddr  = t.c.LocalAddr()
		remoteAddr = t.c.RemoteAddr()
	)
	if readBuffer > 0 {
		r = bufio.NewReaderSize(r, readBuffer)
	}
	// 寫入 tcp-chain
	go t.write(writeBuffer)

	// 讀取 tcp-chain
TS:
	for {
		// 讀取指令
		_, e = io.ReadFull(r, b[:1])
		if e != nil {
			break
		}
		cmd := Command(b[0])
		switch cmd {
		case CommandPing:
		case CommandPong: // 響應 pong
			_, e = io.ReadFull(r, b[:4])
			if e != nil {
				break TS
			}
			data := make([]byte, 5)
			data[0] = byte(CommandPong)
			copy(data[1:], b[:4])
			select {
			case <-t.done:
				break TS
			case ch <- data:
			}
			if t.send(data) {
				break TS
			}
		case CommandCreate: // 創建 channel
			_, e = io.ReadFull(r, b[:8])
			if e != nil {
				break TS
			}
			id := binary.BigEndian.Uint64(b)

			data := make([]byte, 1+8+1)
			data[0] = byte(CommandCreate)
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
				t.keys[id] = val
				data[1+8] = 0
			}
			t.Unlock()
			if t.send(data) {
				break TS
			}
		case CommandClose: // 客戶端要求關閉 channel
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
		case CommandWrite: // 向 channel 寫入數據
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
				ServerLogger.Printf(CommandWrite.String()+": channel(%v) not found\n", id)
			}
		case CommandConfirm: // 確認 channel 數據
			_, e = io.ReadFull(r, b[:8+4])
			if e != nil {
				break TS
			}
			id := binary.BigEndian.Uint64(b)
			t.Lock()
			sc, exists := t.keys[id]
			t.Unlock()
			if exists {
				if sc.Confirm(int64(binary.BigEndian.Uint32(b[8:]))) {
					t.sendClose(id)
					ServerLogger.Printf(CommandConfirm.String()+": channel(%v) overflow\n", id)
				}
			} else {
				t.sendClose(id)
				ServerLogger.Printf(CommandConfirm.String()+": channel(%v) not found\n", id)
			}
		default:
			ServerLogger.Println(cmd.String())
			break TS
		}
	}
	// 清理 channel
	t.Lock()
	for _, c := range keys {
		c.Close()
	}
	t.Unlock()
}
func (t *serverTransport) send(b []byte) (exit bool) {
	select {
	case <-t.done:
		exit = true
		return
	case t.ch <- b:
		return
	default:
	}
	go func() {
		select {
		case <-t.done:
			return
		case t.ch <- b:
			return
		}
	}()
	return
}
func (t *serverTransport) write(writeBuffer int) {
	defer t.Close()
	var (
		b  []byte
		w  = bufio.NewWriterSize(t.c, writeBuffer)
		e  error
		ch = t.ch
	)
	for {
		// 讀取待寫入數據
		select {
		case b = <-ch:
			_, e = w.Write(b)
			if e != nil {
				return
			}
		case <-t.done:
			return
		}

		// 合併剩餘數據
	FM:
		for {
			select {
			case b = <-ch:
				_, e = w.Write(b)
				if e != nil {
					return
				}
			case <-t.done:
				return
			default:
				break FM
			}
		}
		// 刷新剩餘數據
		e = w.Flush()
		if e != nil {
			break
		}
	}
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
	b[0] = byte(CommandClose)
	binary.BigEndian.PutUint64(b[1:], id)
	select {
	case <-t.done:
	case t.ch <- b:
	}
}
