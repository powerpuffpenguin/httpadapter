package httpadapter_test

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/powerpuffpenguin/httpadapter"
	"github.com/stretchr/testify/assert"
)

const Addr = "127.0.0.1:12233"

func newHTTP(t *testing.T) (s *httpadapter.Server, l net.Listener) {
	l, e := net.Listen(`tcp`, Addr)
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	mux := http.NewServeMux()
	mux.HandleFunc(`/version`, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(`Content-Type`, `text/plain; charset=utf-8`)
		w.Write([]byte(fmt.Sprintf("version=%v\nprotocol=%v\n",
			httpadapter.Version,
			httpadapter.ProtocolVersion,
		)))
	})
	s = httpadapter.NewServer(
		httpadapter.ServerHTTP(mux),
	)

	return
}

var testErrors = []struct {
	Window  uint16
	Version []string
	Code    httpadapter.Hello
	Match   string
}{
	{
		Window:  0,
		Version: []string{"1.0"},
		Code:    httpadapter.HelloWindow,
	},
	{
		Window:  1,
		Version: []string{"0.15"},
		Code:    httpadapter.HelloInvalidVersion,
	},
	{
		Window:  1,
		Version: []string{"0.15", httpadapter.ProtocolVersion},
		Code:    httpadapter.HelloOk,
		Match:   httpadapter.ProtocolVersion,
	},
}

func TestServer(t *testing.T) {
	s, l := newHTTP(t)
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		s.Serve(l)
		wait.Done()
	}()

	resp, e := http.Get(`http://` + Addr + "/version")
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	resp.Body.Close()
	if !assert.Equal(t, http.StatusOK, resp.StatusCode) {
		t.FailNow()
	}

	resp, e = http.Get(`http://` + Addr + "/notfound")
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	resp.Body.Close()
	if !assert.Equal(t, http.StatusNotFound, resp.StatusCode) {
		t.FailNow()
	}
	for _, node := range testErrors {
		vs := strings.Join(node.Version, ",")
		b := make([]byte, len(httpadapter.Flag)+2+2+len(vs))
		i := copy(b, httpadapter.Flag)
		binary.BigEndian.PutUint16(b[i:], node.Window)
		i += 2
		binary.BigEndian.PutUint16(b[i:], uint16(len(vs)))
		i += 2
		copy(b[i:], []byte(vs))

		c, e := net.Dial(`tcp`, Addr)
		if !assert.Nil(t, e) {
			t.FailNow()
		}
		_, e = c.Write(b)
		if !assert.Nil(t, e) {
			t.FailNow()
		}

		min := len(httpadapter.Flag) + 1 + 2 + 2
		_, e = io.ReadAtLeast(c, b[:min], min)
		if !assert.Nil(t, e) {
			t.FailNow()
		}
		i = len(httpadapter.Flag)
		if !assert.Equal(t, httpadapter.Flag, string(b[:i])) {
			t.FailNow()
		}
		if !assert.Equal(t, node.Code, httpadapter.Hello(b[i])) {
			t.FailNow()
		}
		i++
		var str string
		if node.Code == httpadapter.HelloOk {
			window := binary.BigEndian.Uint16(b[i:])
			if !assert.Equal(t, s.Window(), window) {
				t.FailNow()
			}
			i += 2

			str = node.Match
		} else {
			if !assert.Equal(t, uint16(0), binary.BigEndian.Uint16(b[i:])) {
				t.FailNow()
			}
			i += 2

			str = node.Code.String()
		}

		if !assert.Equal(t, uint16(len(str)), binary.BigEndian.Uint16(b[i:])) {
			t.FailNow()
		}

		if len(b) < len(str) {
			b = make([]byte, len(str))
		} else {
			b = b[:len(str)]
		}
		_, e = io.ReadAtLeast(c, b, len(b))
		if !assert.Nil(t, e) {
			t.FailNow()
		}
		if !assert.Equal(t, str, string(b)) {
			t.FailNow()
		}
		c.Close()
	}
	l.Close()
	wait.Wait()
}
