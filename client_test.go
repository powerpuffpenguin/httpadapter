package httpadapter_test

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/powerpuffpenguin/httpadapter"
	"github.com/powerpuffpenguin/httpadapter/core"
	"github.com/stretchr/testify/assert"
)

func ServerEcho(duration time.Duration) httpadapter.ServerOption {
	return httpadapter.ServerHandler(httpadapter.HandleFunc(func(c httpadapter.Conn) {
		defer c.Close()
		b := make([]byte, 10)
		for {
			n, e := c.Read(b)
			if e != nil {
				break
			}
			if duration > 0 {
				time.Sleep(duration)
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
		ServerEcho(0),
		httpadapter.ServerWindow(4),
	)
	defer s.CloseAndWait()

	client := httpadapter.NewClient(Addr)

	for i := 0; i < 10; i++ {
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
}
func TestClientSleep(t *testing.T) {
	s := newServer(t,
		ServerEcho(time.Millisecond),
		httpadapter.ServerWindow(4),
	)
	defer s.CloseAndWait()

	client := httpadapter.NewClient(Addr)

	for i := 0; i < 10; i++ {
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

}

func TestClientHttp(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(`/version`, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(`Content-Type`, `text/plain; charset=utf-8`)
		w.Write([]byte(fmt.Sprintf("version=%v\nprotocol=%v\n",
			core.Version,
			core.ProtocolVersion,
		)))
	})

	s := newServer(t,
		httpadapter.ServerWindow(4),
		httpadapter.ServerHTTP(mux),
	)
	defer s.CloseAndWait()

	client := httpadapter.NewClient(Addr)

	client.Dial()

}
