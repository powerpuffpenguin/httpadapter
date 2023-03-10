package httpadapter

import (
	"net"
	"sync"
)

type httpListner struct {
	done chan struct{}
	ch   chan net.Conn
	addr net.Addr
}

// Accept waits for and returns the next connection to the listener.
func (l *httpListner) Accept() (net.Conn, error) {
	select {
	case c := <-l.ch:
		return c, nil
	case <-l.done:
		return nil, ErrServerClosed
	}
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (l *httpListner) Close() error {
	return nil
}

// Addr returns the listener's network address.
func (l *httpListner) Addr() net.Addr {
	return l.addr
}

type httpConn struct {
	net.Conn
	b      []byte
	locker sync.Mutex
}

func (c *httpConn) Read(b []byte) (n int, err error) {
	c.locker.Lock()
	if len(c.b) == 0 {
		n, err = c.Conn.Read(b)
	} else {
		n = copy(b, c.b)
		c.b = c.b[n:]
	}
	c.locker.Unlock()
	return
}
