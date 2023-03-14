package httpadapter

import (
	"context"
	"encoding/binary"
	"net"
	"sync/atomic"
	"time"
)

type dataTransport interface {
	delete(c *dataChannel)
	Done() <-chan struct{}
	getWriter() chan<- []byte
}
type dataChannel struct {
	id            uint64
	t             dataTransport
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

func newChannel(t dataTransport,
	id uint64,
	localAddr, remoteAddr net.Addr,
	window, remoteWindow int,
) *dataChannel {
	done := make(chan struct{})
	return &dataChannel{
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
func (c *dataChannel) Close() (e error) {
	if c.closed == 0 && atomic.SwapInt32(&c.closed, 1) == 0 {
		close(c.done)
		c.pipe.Close()
	} else {
		e = ErrChannelClosed
	}
	return
}
func (c *dataChannel) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *dataChannel) RemoteAddr() net.Addr {
	return c.remoteAddr
}
func (c *dataChannel) SetDeadline(t time.Time) (e error) {
	if c.closed == 0 && atomic.LoadInt32(&c.closed) == 0 {
		c.deadline.Store(t)
	} else {
		e = ErrChannelClosed
	}
	return
}

func (c *dataChannel) SetReadDeadline(t time.Time) (e error) {
	if c.closed == 0 && atomic.LoadInt32(&c.closed) == 0 {
		c.readDeadline.Store(t)
	} else {
		e = ErrChannelClosed
	}
	return
}

func (c *dataChannel) SetWriteDeadline(t time.Time) (e error) {
	if c.closed == 0 && atomic.LoadInt32(&c.closed) == 0 {
		c.writeDeadline.Store(t)
	} else {
		e = ErrChannelClosed
	}
	return
}
func (c *dataChannel) Serve() {
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
		done      = c.t.Done()
		ch        = c.t.getWriter()
	)
	for {
		b, confirm, exit = c.choose(b)
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
			case <-done:
				return
			case <-c.done:
				return
			case ch <- b[:size]:
				writed += size
				b = b[size:]
			}
		}
	}
}
func (c *dataChannel) choose(b []byte) (data []byte, confirm int, exit bool) {
	done := c.t.Done()
	if len(b) == 0 {
		select {
		case <-done:
			exit = true
		case <-c.done:
			exit = true
		case data = <-c.write:
		case confirm = <-c.confirm:
		}
	} else {
		data = b
		select {
		case <-done:
			exit = true
		case <-c.done:
			exit = true
		case confirm = <-c.confirm:
		default:
		}
	}
	return
}
func (c *dataChannel) Write(b []byte) (n int, e error) {
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
		case <-c.t.Done():
			e = ErrTCPClosed
		case <-c.done:
			e = ErrChannelClosed
		case c.write <- data:
			n = len(data)
		}
	} else {
		select {
		case <-c.t.Done():
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
func (c *dataChannel) Confirm(val int) (overflow bool) {
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
func (c *dataChannel) serveConfirm() {
	var (
		v0, v1 int
		window = c.remoteWindow
		done   = c.t.Done()
		ch     = c.t.getWriter()
	)
	for {
		select {
		case <-c.done:
			return
		case <-done:
			return
		case v0 = <-c.sendConfirm:
		}

	CSF:
		for v0 < window {
			select {
			case <-c.done:
				return
			case <-done:
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
		case <-done:
			return
		case ch <- data:
		}
	}
}
func (c *dataChannel) Read(b []byte) (n int, e error) {
	n, e = c.pipe.Read(b)
	if e == nil && n != 0 {
		select {
		case c.sendConfirm <- n:
		case <-c.done:
		case <-c.t.Done():
		}
	}
	return
}
func (c *dataChannel) Pipe(b []byte) {
	_, e := c.pipe.Write(b)
	if e != nil { // pipe 錯誤關閉 channel
		c.Close()
	}
}