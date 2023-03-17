package httpadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
		for _, v := range r.Header.Values(`Accept`) {
			if strings.HasPrefix(v, `application/json`) {
				w.Header().Set(`Content-Type`, `application/json; charset=utf-8`)
				json.NewEncoder(w).Encode(map[string]string{
					`version`:  core.Version,
					`protocol`: core.ProtocolVersion,
				})
				return
			}
		}
		w.Header().Set(`Content-Type`, `text/plain; charset=utf-8`)
		w.Write([]byte(fmt.Sprintf("version=%v\nprotocol=%v",
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

	// bad gateway
	resp, e := client.Unary(context.Background(), &httpadapter.UnaryRequest{
		URL:    `abc`,
		Method: http.MethodPost,
	})
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	resp.Body.Close()
	if !assert.Equal(t, http.StatusBadGateway, resp.Status) {
		t.FailNow()
	}

	// 404
	resp, e = client.Unary(context.Background(), &httpadapter.UnaryRequest{
		URL:    BaseURL + `/404`,
		Method: http.MethodGet,
	})
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	resp.Body.Close()
	if !assert.Equal(t, http.StatusNotFound, resp.Status) {
		t.FailNow()
	}

	// text/plain
	resp, e = client.Unary(context.Background(), &httpadapter.UnaryRequest{
		URL:    BaseURL + `/version`,
		Method: http.MethodGet,
	})
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	if !assert.Equal(t, http.StatusOK, resp.Status) {
		t.FailNow()
	}
	if !assert.Equal(t, resp.Header.Get(`Content-Type`), `text/plain; charset=utf-8`) {
		t.FailNow()
	}

	b, e := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	if !assert.Equal(t, string(b), fmt.Sprintf("version=%v\nprotocol=%v",
		core.Version,
		core.ProtocolVersion,
	)) {
		t.FailNow()
	}

	// json
	resp, e = client.Unary(context.Background(), &httpadapter.UnaryRequest{
		URL:    BaseURL + `/version`,
		Method: http.MethodGet,
		Header: http.Header{
			`Accept`: []string{`application/json`},
		},
	})
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	if !assert.Equal(t, http.StatusOK, resp.Status) {
		t.FailNow()
	}
	if !assert.Equal(t, resp.Header.Get(`Content-Type`), `application/json; charset=utf-8`) {
		t.FailNow()
	}
	b, e = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	if !assert.Equal(t, string(b), fmt.Sprintf(`{"protocol":"%v","version":"%v"}`+"\n",
		core.ProtocolVersion,
		core.Version,
	)) {
		t.FailNow()
	}
}
