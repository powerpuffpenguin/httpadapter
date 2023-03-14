package httpadapter_test

import (
	"io"
	"net"
	"testing"

	"github.com/powerpuffpenguin/httpadapter"
	"github.com/powerpuffpenguin/httpadapter/core"
	"github.com/stretchr/testify/assert"
)

func ServerEcho() httpadapter.ServerOption {
	return httpadapter.ServerHandler(httpadapter.HandleFunc(func(c net.Conn) {
		defer c.Close()
		b := make([]byte, 10)
		for {
			n, e := c.Read(b)
			if e != nil {
				break
			}
			_, e = c.Write(b[:n])
			if e != nil {
				break
			}
		}
	}))
}
func TestClient(t *testing.T) {
	s := newServer(t,
		ServerEcho(),
		httpadapter.ServerWindow(4),
	)
	defer s.CloseAndWait()

	client := httpadapter.NewClient(Addr)
	c, e := client.Dial()
	if !assert.Nil(t, e) {
		t.FailNow()
	}

	b := make([]byte, 8)
	for i := uint64(0); i < 10; i++ {
		core.ByteOrder.PutUint64(b, i)
		_, e = c.Write(b)
		if !assert.Nil(t, e) {
			t.FailNow()
		}
		n, e := io.ReadFull(c, b)
		if !assert.Nil(t, e) {
			t.FailNow()
		}
		if !assert.Equal(t, n, 8) {
			t.FailNow()
		}
	}
	c.Close()

}
