package httpadapter_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/powerpuffpenguin/httpadapter"
	"github.com/powerpuffpenguin/httpadapter/core"
	"github.com/stretchr/testify/assert"
)

const Addr = "127.0.0.1:12233"
const BaseURL = "http://" + Addr
const BaseWebsocket = "ws://" + Addr

type _Server struct {
	*httpadapter.Server
	done chan struct{}
}

func newServer(t *testing.T, opt ...httpadapter.ServerOption) *_Server {
	s, l := newHTTP(t, opt...)
	done := make(chan struct{})
	go func() {
		s.Serve(l)
		close(done)
	}()
	return &_Server{
		Server: s,
		done:   done,
	}
}
func (s *_Server) CloseAndWait() {
	s.Server.Close()
	<-s.done
	http.DefaultClient.CloseIdleConnections()
}

func newHTTP(t *testing.T, opt ...httpadapter.ServerOption) (s *httpadapter.Server, l net.Listener) {
	l, e := net.Listen(`tcp`, Addr)
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	s = httpadapter.NewServer(
		opt...,
	)
	return
}
func TestServerHTTP(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(`/version`, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(`Content-Type`, `text/plain; charset=utf-8`)
		w.Write([]byte(fmt.Sprintf("version=%v\nprotocol=%v\n",
			core.Version,
			core.ProtocolVersion,
		)))
	})
	s := newServer(t,
		httpadapter.ServerHTTP(mux),
	)
	defer s.CloseAndWait()

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

	hello := core.ClientHello{
		Window:  1,
		Version: []string{"noany"},
	}
	b, e := hello.Marshal()
	if !assert.Nil(t, e) {
		t.FailNow()
	}

	c, e := net.Dial(`tcp`, Addr)
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	defer c.Close()
	_, e = c.Write(b)
	if !assert.Nil(t, e) {
		t.FailNow()
	}

	sh, e := core.ReadServerHello(c, nil)
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	if !assert.Equal(t, sh.Window, s.Window()) {
		t.FailNow()
	}
	if !assert.Equal(t, sh.Code, core.HelloInvalidVersion) {
		t.FailNow()
	}
}

var testErrors = []struct {
	Flag    string
	Window  uint16
	Version []string
	Code    core.Hello
	Match   string
}{
	{
		Window:  0,
		Version: []string{"1.0"},
		Code:    core.HelloInvalidWindow,
	},
	{
		Window:  1,
		Version: []string{"0.15"},
		Code:    core.HelloInvalidVersion,
	},
	{
		Window:  1,
		Version: []string{"0.15", core.ProtocolVersion},
		Code:    core.HelloOk,
		Match:   core.ProtocolVersion,
	},
	{
		Flag:    `Httpadapter`,
		Window:  1,
		Version: []string{core.ProtocolVersion},
		Code:    core.HelloInvalidProtocol,
	},
}

func TestServer(t *testing.T) {
	s := newServer(t)
	defer s.CloseAndWait()

	for _, node := range testErrors {
		vs := strings.Join(node.Version, ",")
		b := make([]byte, len(core.Flag)+2+2+len(vs))
		flag := node.Flag
		if flag == `` {
			flag = core.Flag
		}
		i := copy(b, flag)
		core.ByteOrder.PutUint16(b[i:], node.Window)
		i += 2
		core.ByteOrder.PutUint16(b[i:], uint16(len(vs)))
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

		min := len(core.Flag) + 1 + 2 + 2
		_, e = io.ReadAtLeast(c, b[:min], min)
		if !assert.Nil(t, e) {
			t.FailNow()
		}
		i = len(core.Flag)
		if !assert.Equal(t, core.Flag, string(b[:i])) {
			t.FailNow()
		}
		if !assert.Equal(t, node.Code, core.Hello(b[i])) {
			t.FailNow()
		}
		i++

		window := core.ByteOrder.Uint16(b[i:])
		if !assert.Equal(t, s.Window(), window) {
			t.FailNow()
		}
		i += 2
		var str string
		if node.Code == core.HelloOk {
			str = node.Match
		} else {
			str = node.Code.String()
		}

		if !assert.Equal(t, uint16(len(str)), core.ByteOrder.Uint16(b[i:])) {
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
}
