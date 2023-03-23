package httpadapter

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/powerpuffpenguin/httpadapter/core"
)

type keyClientChannel struct {
	channel *ioChannel
	rw      *keyClientChannelRW
}
type keyClientChannelRW struct {
	ctx context.Context
	req []byte
	ch  chan createClientChannel
}
type createClientChannel struct {
	value *ioChannel
	code  byte
}
type clientTransport struct {
	// 已經使用的 id，這個值並不準確，只是爲了不用加鎖預估是否還有可用 id
	used uint64
	// 下一個 channel 的 id
	id   uint64
	opts *clientOptions
	keys map[uint64]*keyClientChannel

	sync.Mutex
	baseTransport
}

func newClientTransport(c net.Conn, buf []byte, opts *clientOptions) (t *clientTransport, e error) {
	req := core.ClientHello{
		Window:  opts.window,
		Version: []string{core.ProtocolVersion},
	}
	data, e := req.MarshalTo(buf)
	if e != nil {
		return
	}
	_, e = c.Write(data)
	if e != nil {
		return
	}

	resp, e := core.ReadServerHello(c, buf)
	if e != nil {
		return
	}
	if resp.Code == core.HelloOk {
		if resp.Message != core.ProtocolVersion {
			e = core.HelloError(core.HelloInvalidVersion)
			return
		}
	} else {
		e = fmt.Errorf("%v %s", resp.Code, resp.Message)
		return
	}
	t = &clientTransport{
		used: 1,
		id:   0,
		opts: opts,
		keys: make(map[uint64]*keyClientChannel),
		baseTransport: baseTransport{
			done:   make(chan struct{}),
			window: resp.Window,
			c:      c,
			ch:     make(chan []byte, 50),
		},
	}
	return
}

func (t *clientTransport) Serve(b []byte) {
	defer t.Close()
	var (
		r          io.Reader = t.c
		e          error
		localAddr  = t.c.LocalAddr()
		remoteAddr = t.c.RemoteAddr()
		active     chan int
	)
	// 建立讀取緩存
	if t.opts.readBuffer > 0 {
		r = bufio.NewReaderSize(r, t.opts.readBuffer)
	}
	// ping
	if t.opts.ping > time.Second {
		active = make(chan int, 1)
		go t.servePing(active, t.opts.ping)
	}
	// 寫入 tcp-chain
	go t.serveWrite(active, t.opts.writeBuffer)

	// 讀取 tcp-chain
CS:
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
				break CS
			}
		case core.CommandCreate:
			_, e = io.ReadFull(r, b[:9])
			if e != nil {
				break CS
			}
			id := core.ByteOrder.Uint64(b)
			t.Lock()
			val, exists := t.keys[id]
			if !exists {
				t.Unlock()
				Logger.Println(`on command:`, cmd, `, id not exists`, id)

				t.sendClose(id) // 對面 channel id 可能不同步，通知它關閉以試圖修復
				continue CS
			} else if val.channel != nil {
				delete(t.keys, id)
				t.Unlock()

				// 對面 channel id 可能不同步，通知它關閉以試圖修復
				val.channel.Close()
				t.sendClose(id)
				continue CS
			}
			rw := val.rw
			if rw.ctx.Err() != nil { // 上層調用者已經取消此 channel
				delete(t.keys, id)
				t.Unlock()

				val.channel.Close()
				t.sendClose(id)
				continue CS
			}
			t.Unlock()

			// 創建 channel
			val.rw = nil
			code := b[8]
			if code == 0 {
				val.channel = newIOChannel(t, id,
					localAddr, remoteAddr,
					int(t.opts.window), int(t.window),
				)
				go val.channel.Serve()
			}
			if t.createResult(rw, code, val.channel) {
				break CS
			}
		case core.CommandClose: // 客戶端要求關閉 channel
			_, e = io.ReadFull(r, b[:8])
			if e != nil {
				break CS
			}
			id := core.ByteOrder.Uint64(b)
			t.Lock()
			if cc, exists := t.keys[id]; exists {
				if cc.channel != nil {
					cc.channel.Close()
				}
				delete(t.keys, id)
			}
			t.Unlock()
		case core.CommandWrite: // 向 channel 寫入數據
			_, e = io.ReadFull(r, b[:8+2])
			if e != nil {
				break CS
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
					break CS
				}
				size -= n
				t.Lock()
				val, exists := t.keys[id]
				t.Unlock()
				if exists {
					if val.channel == nil {
						if healthy {
							healthy = false
							t.sendClose(id)
							Logger.Printf(core.CommandConfirm.String()+": channel(%v) not ready\n", id)
						}
					} else {
						val.channel.Pipe(buffer[:n])
					}
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
				break CS
			}
			id := core.ByteOrder.Uint64(b)
			t.Lock()
			val, exists := t.keys[id]
			t.Unlock()
			if exists {
				if val.channel == nil {
					t.sendClose(id)
					Logger.Printf(core.CommandConfirm.String()+": channel(%v) not ready\n", id)
				} else if val.channel.Confirm(uint64(core.ByteOrder.Uint16(b[8:]))) {
					val.channel.Close()
					t.sendClose(id)
					Logger.Printf(core.CommandConfirm.String()+": channel(%v) overflow\n", id)
				}
			} else {
				t.sendClose(id)
				// Logger.Printf(core.CommandConfirm.String()+": channel(%v) not found\n", id)
			}
		default:
			Logger.Println(`Unknow Command:`, cmd.String())
			break CS
		}
	}

	// 清理 channel
	t.Lock()
	for _, c := range t.keys {
		if c.channel != nil {
			c.channel.Close()
		}
	}
	t.Unlock()
}

func (t *clientTransport) delete(c *ioChannel) {
	deleted := false
	t.Lock()
	if val, exists := t.keys[c.id]; exists && val.channel == c {
		delete(t.keys, c.id)
		deleted = true
	}
	t.Unlock()
	if !deleted {
		return
	}
	t.sendClose(c.id)
}

func (t *clientTransport) createResult(rw *keyClientChannelRW, code byte, val *ioChannel) (exit bool) {
	select {
	case <-rw.ctx.Done():
		if code == 0 {
			val.Close()
			t.sendClose(val.id)
		}
		return
	case <-t.done:
		exit = true
		return
	case rw.ch <- createClientChannel{
		code:  code,
		value: val,
	}:
		return
	default:
	}

	go func() {
		select {
		case <-rw.ctx.Done():
			if code == 0 {
				val.Close()
				t.sendClose(val.id)
			}
		case <-t.done:
		case rw.ch <- createClientChannel{
			code:  code,
			value: val,
		}:
		}
	}()
	return
}
func (t *clientTransport) Create(ctx context.Context) (c net.Conn, e error) {
	id := atomic.AddUint64(&t.id, 1)
	data := make([]byte, 1+8)
	data[0] = byte(core.CommandCreate)
	core.ByteOrder.PutUint64(data[1:], id)

	ch := make(chan createClientChannel)
	// 標記請求
	t.Lock()
	e = ctx.Err()
	if e != nil {
		t.Unlock()
		return
	}
	t.keys[id] = &keyClientChannel{
		rw: &keyClientChannelRW{
			req: data,
			ctx: ctx,
			ch:  ch,
		},
	}
	t.Unlock()

	// 發送請求
	select {
	case <-ctx.Done():
		e = ctx.Err()
		return
	case <-t.done:
		e = ErrClientClosed
		return
	case t.ch <- data:
	}
	// 等待響應
	select {
	case <-ctx.Done():
		e = ctx.Err()
		return
	case <-t.done:
		e = ErrClientClosed
	case val := <-ch:
		switch val.code {
		case 0:
			c = val.value
		case 1:
			e = errors.New(`code=1 id already exists: ` + strconv.FormatInt(int64(id), 10))
		case 2:
			e = errors.New(`code=2 too many channels`)
		default:
			e = errors.New(`unknow error(` + strconv.Itoa(int(val.code)) + `)`)
		}
	}
	return
}
