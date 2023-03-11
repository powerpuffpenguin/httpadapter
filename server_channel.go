package httpadapter

import (
	"context"
	"encoding/binary"
	"net"
	"sync/atomic"
	"time"
)

type serverChannel struct {
	id            uint64
	t             *serverTransport
	localAddr     net.Addr
	remoteAddr    net.Addr
	closed        int32
	done          chan struct{}
	write         chan []byte
	pipe          *pipeReader
	remoteWindow  int
	confirm       chan int
	sendConfirm   chan int
	deadline      atomic.Value
	readDeadline  atomic.Value
	writeDeadline atomic.Value
}

func newServerChannel(t *serverTransport,
	id uint64,
	localAddr, remoteAddr net.Addr,
	window, remoteWindow int,
) *serverChannel {
	done := make(chan struct{})
	return &serverChannel{
		id:           id,
		t:            t,
		localAddr:    localAddr,
		remoteAddr:   remoteAddr,
		done:         done,
		write:        make(chan []byte),
		pipe:         newPipeReader(window),
		remoteWindow: remoteWindow,
		confirm:      make(chan int, 1),
		sendConfirm:  make(chan int, 10),
	}
}
func (c *serverChannel) Close() (e error) {
	if c.closed == 0 && atomic.SwapInt32(&c.closed, 1) == 0 {
		close(c.done)
		c.pipe.Close()
	} else {
		e = ErrChannelClosed
	}
	return
}
func (c *serverChannel) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *serverChannel) RemoteAddr() net.Addr {
	return c.remoteAddr
}
func (c *serverChannel) SetDeadline(t time.Time) (e error) {
	if c.closed == 0 && atomic.LoadInt32(&c.closed) == 0 {
		c.deadline.Store(t)
	} else {
		e = ErrChannelClosed
	}
	return
}

func (c *serverChannel) SetReadDeadline(t time.Time) (e error) {
	if c.closed == 0 && atomic.LoadInt32(&c.closed) == 0 {
		c.readDeadline.Store(t)
	} else {
		e = ErrChannelClosed
	}
	return
}

func (c *serverChannel) SetWriteDeadline(t time.Time) (e error) {
	if c.closed == 0 && atomic.LoadInt32(&c.closed) == 0 {
		c.writeDeadline.Store(t)
	} else {
		e = ErrChannelClosed
	}
	return
}
func (c *serverChannel) Serve() {
	defer func() {
		// 關閉 channel
		if c.Close() == nil {
			// 通知 tcp-chain 關閉
			c.t.delete(c)
		}
	}()
	go c.serveConfirm()
	var (
		b         []byte
		exit      bool
		confirm   int // 對方確認收到的數據
		writed    = 0 // 已經寫入的數據
		available int // 可寫數據
		size      int
	)
	for {
		b, confirm, exit = c.choose(b)
		if exit {
			break
		} else if confirm > 0 {
			if confirm > writed {
				ServerLogger.Printf("channel(%v) confirm(%v) > writed(%v)\n", c.id, confirm, writed)
				break
			} else {
				writed -= confirm
			}
		}
		size = len(b)
		for size != 0 {
			available = c.remoteWindow - writed
			if available == 0 {
				break
			}
			if size > available {
				size = available
			}
			select {
			case <-c.t.done:
				return
			case <-c.done:
				return
			case c.t.ch <- b[:size]:
				writed += size
				b = b[size:]
			}
		}
	}
}
func (c *serverChannel) choose(b []byte) (data []byte, confirm int, exit bool) {
	if len(b) == 0 {
		select {
		case <-c.t.done:
			exit = true
		case <-c.done:
			exit = true
		case data = <-c.write:
		case confirm = <-c.confirm:
		}
	} else {
		data = b
		select {
		case <-c.t.done:
			exit = true
		case <-c.done:
			exit = true
		case confirm = <-c.confirm:
		default:
		}
	}
	return
}
func (c *serverChannel) Write(b []byte) (n int, e error) {
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
	if deadline.IsZero() {
		now := time.Now()
		if deadline.After(now) {
			timer = time.NewTimer(deadline.Sub(now))
		} else {
			e = context.DeadlineExceeded
			return
		}
	}

	data := make([]byte, len(b))
	copy(data, b)
	if timer == nil {
		select {
		case <-c.t.done:
			e = ErrTCPClosed
		case <-c.done:
			e = ErrChannelClosed
		case c.write <- data:
			n = len(data)
		}
	} else {
		select {
		case <-c.t.done:
			if !timer.Stop() {
				<-timer.C
			}
			e = ErrTCPClosed
		case <-c.done:
			if !timer.Stop() {
				<-timer.C
			}
			e = ErrChannelClosed
		case c.write <- data:
			if !timer.Stop() {
				<-timer.C
			}
			n = len(data)
		case <-timer.C:
			e = context.DeadlineExceeded
		}
	}
	return
}
func (c *serverChannel) Confirm(val int) (overflow bool) {
	select {
	case <-c.done:
		return
	case c.confirm <- val:
		return
	default:
	}

	for {
		select {
		case <-c.done:
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
func (c *serverChannel) serveConfirm() {
	var (
		v0, v1 int
		window = c.remoteWindow
	)
	for {
		select {
		case <-c.done:
			return
		case <-c.t.done:
			return
		case v0 = <-c.sendConfirm:
		}

	CSF:
		for v0 < window {
			select {
			case <-c.done:
				return
			case <-c.t.done:
				return
			case v1 = <-c.sendConfirm:
				v0 += v1
			default:
				break CSF
			}
		}
		data := make([]byte, 1+8+2)
		data[0] = 6
		binary.BigEndian.PutUint64(data[1:], c.id)
		binary.BigEndian.PutUint16(data[1+8:], uint16(v0))
		select {
		case <-c.done:
			return
		case <-c.t.done:
			return
		case c.t.ch <- data:
		}
	}
}
func (c *serverChannel) Read(b []byte) (n int, e error) {
	n, e = c.pipe.Read(b)
	if e == nil && n != 0 {
		select {
		case c.sendConfirm <- n:
		case <-c.done:
		case <-c.t.done:
		}
	}
	return
}
func (c *serverChannel) Pipe(b []byte) {
	_, e := c.pipe.Write(b)
	if e != nil { // pipe 錯誤關閉 channel
		c.Close()
	}
}
