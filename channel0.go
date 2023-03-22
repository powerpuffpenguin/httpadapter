package httpadapter

import (
	"context"
	"math"
	"net"
	"sync/atomic"
	"time"

	"github.com/powerpuffpenguin/httpadapter/core"
	"github.com/powerpuffpenguin/httpadapter/internal/memory"
)

type ioTransport interface {
	delete(c *ioChannel)
	Done() <-chan struct{}
	getWriter() chan<- memory.Buffer
	getAllocator() memory.BufferAllocator
}
type ioChannel struct {
	// channel id
	id uint64
	// 傳輸層
	transport ioTransport
	// 網路地址
	localAddr  net.Addr
	remoteAddr net.Addr

	// 關閉標籤
	closed int32
	ctx    context.Context
	cancel context.CancelFunc

	// 數據寫入通道
	write chan memory.Buffer

	// 讀寫管道
	pipe *pipeReader
	// 對面窗口
	remoteWindow uint64
	// 收到確認包
	confirm chan uint64
	// 發送確認包
	sendConfirm chan int
	// 讀寫截止時間
	deadline atomic.Value
	// 讀取截止時間
	readDeadline atomic.Value
	// 寫入截止時間
	writeDeadline atomic.Value
}

func newIOChannel(transport ioTransport,
	id uint64,
	localAddr, remoteAddr net.Addr,
	window, remoteWindow int,
) *ioChannel {
	ctx, cancel := context.WithCancel(context.Background())
	return &ioChannel{
		transport:    transport,
		id:           id,
		localAddr:    localAddr,
		remoteAddr:   remoteAddr,
		ctx:          ctx,
		cancel:       cancel,
		write:        make(chan memory.Buffer),
		pipe:         newPipeReader(window),
		remoteWindow: uint64(remoteWindow),
		confirm:      make(chan uint64, 1),
		sendConfirm:  make(chan int, 10),
	}
}

func (c *ioChannel) Context() context.Context {
	return c.ctx
}

func (c *ioChannel) Close() (e error) {
	if c.closed == 0 && atomic.SwapInt32(&c.closed, 1) == 0 {
		c.cancel()
		c.pipe.Close()
	} else {
		e = ErrChannelClosed
	}
	return
}

func (c *ioChannel) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *ioChannel) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *ioChannel) SetDeadline(t time.Time) (e error) {
	if c.closed == 0 && atomic.LoadInt32(&c.closed) == 0 {
		c.deadline.Store(t)
	} else {
		e = ErrChannelClosed
	}
	return
}

func (c *ioChannel) SetReadDeadline(t time.Time) (e error) {
	if c.closed == 0 && atomic.LoadInt32(&c.closed) == 0 {
		c.readDeadline.Store(t)
	} else {
		e = ErrChannelClosed
	}
	return
}

func (c *ioChannel) SetWriteDeadline(t time.Time) (e error) {
	if c.closed == 0 && atomic.LoadInt32(&c.closed) == 0 {
		c.writeDeadline.Store(t)
	} else {
		e = ErrChannelClosed
	}
	return
}

func (c *ioChannel) Serve() {
	defer func() {
		// 關閉 channel
		c.Close()
		// 通知 tcp-chain 關閉
		c.transport.delete(c)
	}()
	go c.serveConfirm()

	var (
		b         memory.Buffer
		exit      bool
		confirm   uint64 // 對方確認收到的數據
		writed    uint64 // 已經寫入的數據
		available uint64 // 可寫數據
		size      uint64
		done0     = c.transport.Done()
		done1     = c.ctx.Done()
		ch        = c.transport.getWriter()
		allocator = c.transport.getAllocator()
		data      memory.Buffer
	)
IOS:
	for {
		b, confirm, exit = c.choose(b, writed)
		if exit {
			break
		} else if confirm > 0 {
			if confirm > writed {
				Logger.Printf("channel(%v) confirm(%v) > writed(%v)\n", c.id, confirm, writed)
				break
			} else {
				writed -= confirm
			}
		}
		size = uint64(len(b.Data))
		for size != 0 {
			available = c.remoteWindow - writed
			if available == 0 {
				break
			}
			if size > available {
				size = available
			}
			data = allocator.Get(int(11 + size))
			data.Data[0] = 5
			core.ByteOrder.PutUint64(data.Data[1:], c.id)
			core.ByteOrder.PutUint16(data.Data[9:], uint16(size))
			copy(data.Data[11:], b.Data[:size])
			select {
			case <-done0:
				allocator.Put(data)
				break IOS
			case <-done1:
				allocator.Put(data)
				break IOS
			case ch <- data:
				writed += size
				b.Data = b.Data[size:]
				size = uint64(len(b.Data))
			}
		}
		if b.Buffer != nil && len(b.Data) == 0 {
			allocator.Put(b)
		}
	}
}

func (c *ioChannel) choose(b memory.Buffer, writed uint64) (data memory.Buffer, confirm uint64, exit bool) {
	done := c.transport.Done()
	if len(b.Data) == 0 {
		select {
		case <-done:
			exit = true
		case <-c.ctx.Done():
			exit = true
		case data = <-c.write:
		case confirm = <-c.confirm:
		}
	} else {
		data = b
		available := c.remoteWindow - writed
		if available == 0 {
			select {
			case <-done:
				exit = true
			case <-c.ctx.Done():
				exit = true
			case confirm = <-c.confirm:
			}
		} else {
			select {
			case <-done:
				exit = true
			case <-c.ctx.Done():
				exit = true
			case confirm = <-c.confirm:
			default:
			}
		}
	}
	return
}
func (c *ioChannel) Write(b []byte) (n int, e error) {
	var deadline time.Time
	v := c.deadline.Load()
	if v != nil {
		deadline = v.(time.Time)
	}
	v = c.writeDeadline.Load()
	if v != nil {
		d := v.(time.Time)
		if d.Before(deadline) {
			deadline = d
		}
	}
	var timer *time.Timer
	if !deadline.IsZero() {
		now := time.Now()
		if deadline.After(now) {
			timer = time.NewTimer(deadline.Sub(now))
		} else {
			e = context.DeadlineExceeded
			return
		}
	}
	size := len(b)
	buffer := c.transport.getAllocator().Get(size)
	copy(buffer.Data, b)
	if timer == nil {
		select {
		case <-c.transport.Done():
			e = ErrTCPClosed
		case <-c.ctx.Done():
			e = ErrChannelClosed
		case c.write <- buffer:
			n = size
		}
	} else {
		select {
		case <-c.transport.Done():
			if !timer.Stop() {
				<-timer.C
			}
			e = ErrTCPClosed
		case <-c.ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			e = ErrChannelClosed
		case c.write <- buffer:
			if !timer.Stop() {
				<-timer.C
			}
			n = size
		case <-timer.C:
			e = context.DeadlineExceeded
		}
	}
	return
}

func (c *ioChannel) Confirm(val uint64) (overflow bool) {
	select {
	case <-c.transport.Done():
		return
	case <-c.ctx.Done():
		return
	case c.confirm <- val:
		return
	default:
	}

	for {
		select {
		case <-c.transport.Done():
			return
		case <-c.ctx.Done():
			return
		case c.confirm <- val:
			return
		case old := <-c.confirm:
			val += old
			if val >= c.remoteWindow {
				overflow = true
				return
			}
		}
	}
}

func (c *ioChannel) serveConfirm() {
	var (
		confirmed, ok uint64
		val           int
		window        = c.remoteWindow / 10
		done0         = c.transport.Done()
		done1         = c.ctx.Done()
		ch            = c.transport.getWriter()
		data          []byte
		buffer, send  memory.Buffer
		allocator     = c.transport.getAllocator()
	)
	for {
		select {
		case <-done0:
			return
		case <-done1:
			return
		case val = <-c.sendConfirm:
			confirmed = uint64(val)
		}

	CSF:
		for confirmed < window {
			select {
			case <-done0:
				return
			case <-done1:
				return
			case val = <-c.sendConfirm:
				confirmed += uint64(val)
			default:
				break CSF
			}
		}

		for confirmed != 0 {
			if len(data) < 11 {
				buffer = allocator.Get(1024 * 32)
				data = buffer.Data
			}
			ok = confirmed
			if ok > math.MaxUint16 {
				ok = math.MaxUint16
			} else {
				ok = confirmed
			}
			data[0] = 6
			core.ByteOrder.PutUint64(data[1:], c.id)
			core.ByteOrder.PutUint16(data[1+8:], uint16(ok))
			send = memory.Buffer{
				Data: data[:11],
			}
			data = data[11:]
			if len(data) < 11 {
				send.Buffer = buffer.Buffer
			}
			select {
			case <-c.ctx.Done():
				return
			case <-done0:
				return
			case <-done1:
				return
			case ch <- send:
				confirmed -= ok
			}
		}
	}
}

func (c *ioChannel) Read(b []byte) (n int, e error) {
	n, e = c.pipe.Read(b)
	if n != 0 {
		select {
		case c.sendConfirm <- n:
		case <-c.ctx.Done():
		case <-c.transport.Done():
		}
	}
	return
}

func (c *ioChannel) Pipe(b []byte) {
	_, e := c.pipe.Write(b)
	if e != nil { // pipe 錯誤關閉 channel
		c.Close()
	}
}
