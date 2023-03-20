package httpadapter

import (
	"context"
	"errors"
	"math"
	"net"
	"sync/atomic"
	"time"
)

var ErrClientClosed = errors.New("httpadapter: Client closed")

type getClientTransport struct {
	Error     error
	Transport *clientTransport
}
type Client struct {
	opts    *clientOptions
	address string
	done    chan struct{}
	closed  int32
	ch      chan chan getClientTransport
	remove  chan *clientTransport
}

func NewClient(address string, opt ...ClientOption) (client *Client) {
	var opts = defaultClientOptions
	for _, o := range opt {
		o.Apply(&opts)
	}
	client = &Client{
		opts:    &opts,
		address: address,
		done:    make(chan struct{}),
		ch:      make(chan chan getClientTransport),
		remove:  make(chan *clientTransport),
	}
	go client.serve()
	return
}
func (c *Client) Close() (e error) {
	if c.closed == 0 && atomic.SwapInt32(&c.closed, 1) == 0 {
		close(c.done)
	} else {
		e = ErrServerClosed
	}
	return
}
func (c *Client) Dial() (net.Conn, error) {
	return c.DialContext(context.Background())
}
func (c *Client) DialContext(ctx context.Context) (conn net.Conn, e error) {
	ch := make(chan getClientTransport, 1)
	select {
	case <-ctx.Done():
		e = ctx.Err()
		return
	case <-c.done:
		e = ErrClientClosed
		return
	case c.ch <- ch:
	}
	var t *clientTransport
	select {
	case <-ctx.Done():
		e = ctx.Err()
		return
	case <-c.done:
		e = ErrClientClosed
		return
	case obj := <-ch:
		t, e = obj.Transport, obj.Error
		if e != nil {
			return
		}
	}
	conn, e = t.Create(ctx)
	return
}
func (c *Client) newTransport() (t *clientTransport, e error) {
	conn, e := c.opts.dialer.Dial(`tcp`, c.address)
	if e != nil {
		return
	}

	buf := make([]byte, 128)
	t, e = newClientTransport(conn, buf, c.opts)
	if e != nil {
		conn.Close()
		return
	}
	go t.Serve(buf)
	return
}
func (c *Client) serve() {
	var (
		keys = make(map[*clientTransport]bool)
		t    *clientTransport
		e    error
	)
CS:
	for {
		select {
		case <-c.done:
			break CS
		case key := <-c.remove:
			delete(keys, key)
		case ch := <-c.ch:
			if t != nil {
				select {
				// check healthy
				case <-t.done:
					t = nil
				default:
				}
			}
			if t == nil {
				t, e = c.newTransport()
				if e != nil {
					ch <- getClientTransport{
						Error: e,
					}
					continue CS
				}
				keys[t] = true
			}
			ch <- getClientTransport{
				Transport: t,
			}

			if t.used == math.MaxUint64 {
				t = nil
			} else {
				t.used++
			}
		}
	}
	// 清理連接
	for t := range keys {
		t.Close()
	}
}

// 返回客戶端 channel window 大小
func (c *Client) Window() uint32 {
	return c.opts.window
}

// 返回 tcp-chain 讀取緩衝區大小
func (c *Client) ReadBuffer() int {
	return c.opts.readBuffer
}

// 返回 tcp-chain  寫入緩衝區大小
func (c *Client) WriteBuffer() int {
	return c.opts.writeBuffer
}

// 返回客戶端每隔多久對沒有數據的連接發送 tcp ping，<1 則不會發送
func (c *Client) Ping() time.Duration {
	return c.opts.ping
}

// 返回客戶端如何連接服務器
func (c *Client) Dialer() ClientDialer {
	return c.opts.dialer
}
