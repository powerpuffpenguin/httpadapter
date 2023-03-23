package httpadapter_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/powerpuffpenguin/httpadapter"
	"github.com/powerpuffpenguin/httpadapter/core"
	"github.com/stretchr/testify/assert"
)

func ServerEcho(duration time.Duration) httpadapter.ServerOption {
	return httpadapter.ServerHandler(httpadapter.HandleFunc(func(srv *httpadapter.Server, c httpadapter.Conn) {
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
	testClient(t)
}
func testClient(t *testing.T, opts ...httpadapter.ClientOption) {
	s := newServer(t,
		ServerEcho(0),
		httpadapter.ServerWindow(4),
	)
	defer s.CloseAndWait()

	client := httpadapter.NewClient(Addr, opts...)
	defer client.Close()

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
	testClientSleep(t)
}
func testClientSleep(t *testing.T, opts ...httpadapter.ClientOption) {
	s := newServer(t,
		ServerEcho(time.Millisecond),
		httpadapter.ServerWindow(4),
	)
	defer s.CloseAndWait()

	client := httpadapter.NewClient(Addr, opts...)
	defer client.Close()

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
	testClientHttp(t)
}
func testClientHttp(t *testing.T, opts ...httpadapter.ClientOption) {
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
	client := httpadapter.NewClient(Addr, opts...)
	defer client.Close()

	// bad gateway
	_, e := client.Unary(context.Background(), &httpadapter.MessageRequest{
		URL:    `abc`,
		Method: http.MethodPost,
	})
	if !assert.True(t, strings.HasPrefix(e.Error(), `not support url:`)) {
		t.FailNow()
	}
	// resp.Body.Close()
	// if !assert.Equal(t, http.StatusBadRequest, resp.Status) {
	// 	t.FailNow()
	// }

	// 404
	resp, e := client.Unary(context.Background(), &httpadapter.MessageRequest{
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
	resp, e = client.Unary(context.Background(), &httpadapter.MessageRequest{
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
	resp, e = client.Unary(context.Background(), &httpadapter.MessageRequest{
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
func checkClientHttpBody(t *testing.T, resp *httpadapter.MessageResponse, e error, val string) {
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	defer resp.Body.Close()
	if !assert.Equal(t, http.StatusOK, resp.Status) {
		t.FailNow()
	}

	b, e := io.ReadAll(resp.Body)
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	if !assert.Equal(t, val, string(b)) {
		t.FailNow()
	}

}
func TestClientHttpBody(t *testing.T) {
	testClientHttpBody(t)
}
func testClientHttpBody(t *testing.T, opts ...httpadapter.ClientOption) {
	mux := http.NewServeMux()
	mux.HandleFunc(`/add`, func(w http.ResponseWriter, r *http.Request) {
		var val int
		switch r.Method {
		case http.MethodGet:
			query := r.URL.Query()
			v0, _ := strconv.Atoi(query.Get(`v0`))
			v1, _ := strconv.Atoi(query.Get(`v1`))
			val = v1 + v0
		case http.MethodPost, http.MethodPatch, http.MethodPut:
			if strings.HasPrefix(r.Header.Get(`Content-Type`), `application/json`) {
				var obj struct {
					V0 int `json:"v0"`
					V1 int `json:"v1"`
				}
				e := json.NewDecoder(r.Body).Decode(&obj)
				if e != nil {
					w.WriteHeader(http.StatusBadRequest)
					w.Header().Set(`Content-Type`, `text/plain; charset=utf-8`)
					w.Write([]byte(e.Error()))
					return
				}
				val = obj.V0 + obj.V1
			} else {
				e := r.ParseForm()
				if e != nil {
					w.WriteHeader(http.StatusBadRequest)
					w.Header().Set(`Content-Type`, `text/plain; charset=utf-8`)
					w.Write([]byte(e.Error()))
					return
				}
				v0, _ := strconv.Atoi(r.PostForm.Get(`v0`))
				v1, _ := strconv.Atoi(r.PostForm.Get(`v1`))
				val = v1 + v0
			}
		}
		w.Header().Set(`Content-Type`, `text/plain; charset=utf-8`)
		w.Write([]byte(fmt.Sprint(val)))
	})

	s := newServer(t,
		httpadapter.ServerWindow(4),
		httpadapter.ServerHTTP(mux),
	)
	defer s.CloseAndWait()

	client := httpadapter.NewClient(Addr, opts...)
	defer client.Close()

	resp, e := client.Unary(context.Background(), &httpadapter.MessageRequest{
		URL:    BaseURL + `/add?v0=1&v1=2`,
		Method: http.MethodGet,
	})
	checkClientHttpBody(t, resp, e, `3`)

	v := url.Values{
		`v0`: []string{`3`},
		`v1`: []string{`4`},
	}
	str := v.Encode()
	resp, e = client.Unary(context.Background(), &httpadapter.MessageRequest{
		URL:     BaseURL + `/add`,
		Method:  http.MethodPost,
		Body:    strings.NewReader(str),
		BodyLen: uint64(len(str)),
	})
	checkClientHttpBody(t, resp, e, `7`)

	b, _ := json.Marshal(map[string]int{
		`v0`: 5,
		`v1`: 6,
	})
	resp, e = client.Unary(context.Background(), &httpadapter.MessageRequest{
		URL:     BaseURL + `/add`,
		Method:  http.MethodPost,
		Body:    bytes.NewReader(b),
		BodyLen: uint64(len(b)),
		Header: http.Header{
			`Content-Type`: []string{"application/json"},
		},
	})
	checkClientHttpBody(t, resp, e, `11`)
}
