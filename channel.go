package httpadapter

import (
	"context"
	"math"
	"net"
	"sync/atomic"
	"time"

	"github.com/powerpuffpenguin/httpadapter/core"
	"github.com/powerpuffpenguin/httpadapter/pipe"
)

type ioTransport interface {
	delete(c *ioChannel)
	Done() <-chan struct{}
	getWriter() chan<- []byte
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
	write chan []byte

	// 讀寫管道
	pipe *pipe.PipeReader
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
		write:        make(chan []byte),
		pipe:         pipe.NewPipeReader(window),
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
		b         []byte
		exit      bool
		confirm   uint64 // 對方確認收到的數據
		writed    uint64 // 已經寫入的數據
		available uint64 // 可寫數據
		size      uint64
		done0     = c.transport.Done()
		done1     = c.ctx.Done()
		ch        = c.transport.getWriter()
		data      []byte
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
		size = uint64(len(b))
		for size != 0 {
			available = c.remoteWindow - writed
			if available == 0 {
				break
			}
			if size > available {
				size = available
			}
			if size > math.MaxUint16 {
				size = math.MaxUint16
			}
			data = make([]byte, 11+size)
			data[0] = 5
			core.ByteOrder.PutUint64(data[1:], c.id)
			core.ByteOrder.PutUint16(data[9:], uint16(size))
			copy(data[11:], b[:size])
			select {
			case <-done0:
				break IOS
			case <-done1:
				break IOS
			case ch <- data:
				writed += size
				b = b[size:]
				size = uint64(len(b))
			}
		}
	}
}

func (c *ioChannel) choose(b []byte, writed uint64) (data []byte, confirm uint64, exit bool) {
	done := c.transport.Done()
	if len(b) == 0 {
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
	buffer := make([]byte, size)
	copy(buffer, b)
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
		window        = c.remoteWindow / 3
		done0         = c.transport.Done()
		done1         = c.ctx.Done()
		ch            = c.transport.getWriter()
		data          []byte
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
				data = make([]byte, 1024*32)
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
			select {
			case <-c.ctx.Done():
				return
			case <-done0:
				return
			case <-done1:
				return
			case ch <- data[:11]:
				data = data[11:]
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
