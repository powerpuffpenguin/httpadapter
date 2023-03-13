package httpadapter

import (
	"encoding/binary"
	"net"
	"sync/atomic"
	"time"
)

type dataTransport interface {
	delete(c *dataChannel)
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
}
func (c *dataChannel) serveConfirm() {
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
func (c *dataChannel) Confirm(val int) (overflow bool) {
	return
}
func (c *dataChannel) Pipe(b []byte) {

}
