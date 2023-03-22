package httpadapter_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/powerpuffpenguin/httpadapter"
	"github.com/stretchr/testify/assert"
)

func TestClientTCP(t *testing.T) {
	testClientTCP(t)
	testClientTCP(t, httpadapter.WithAllocator(clientAllocator))
}
func testClientTCP(t *testing.T, opts ...httpadapter.ClientOption) {
	var upgrader = websocket.Upgrader{}
	mux := http.NewServeMux()
	var step int64
	send := make(chan struct{})
	mux.HandleFunc(`/text`, func(w http.ResponseWriter, r *http.Request) {
		v := atomic.SwapInt64(&step, 1)
		if v != 0 {
			w.Write([]byte(fmt.Sprintf(`step=%v step!=0`, v)))
			return
		}
		close(send)
		time.Sleep(time.Millisecond * 50)
		v = atomic.SwapInt64(&step, 2)
		if v != 1 {
			w.Write([]byte(fmt.Sprintf(`step=%v step!=1`, v)))
			return
		}
		w.Write([]byte("ok"))
	})
	var wait sync.WaitGroup
	l, e := net.Listen(`tcp`, TCP)
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	defer func() {
		l.Close()
		wait.Wait()
	}()
	wait.Add(1)
	go func() {
		defer wait.Done()
		for {
			c, e := l.Accept()
			if e != nil {
				break
			}
			go func(c net.Conn) {
				defer func() {
					c.Close()
				}()
				b := make([]byte, 1024)
				for {
					_, e = io.ReadFull(c, b[:2])
					if e != nil {
						break
					}
					n := binary.LittleEndian.Uint16(b)
					_, e = io.ReadFull(c, b[2:2+n])
					if e != nil {
						break
					}
					_, e = c.Write(b[:2+n])
					if e != nil {
						break
					}
				}
			}(c)
		}
	}()

	mux.HandleFunc(`/ws`, func(w http.ResponseWriter, r *http.Request) {
		ws, e := upgrader.Upgrade(w, r, nil)
		if e != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set(`Content-Type`, `text/plain; charset=utf-8`)
			w.Write([]byte(e.Error()))
			return
		}
		defer ws.Close()
		for {
			t, p, e := ws.ReadMessage()
			if e != nil {
				break
			}
			switch t {
			case websocket.TextMessage:
				if !strings.HasPrefix(string(p), "str-") {
					p = []byte("text not start with str-")
				}
			case websocket.BinaryMessage:
				if !bytes.HasPrefix(p, []byte("bytes-")) {
					p = []byte("binary not start with bytes-")
				}
			}
			e = ws.WriteMessage(t, p)
			if e != nil {
				break
			}
		}
	})

	s := newServer(t,
		httpadapter.ServerWindow(4),
		httpadapter.ServerHTTP(mux),
	)
	defer s.CloseAndWait()

	client := httpadapter.NewClient(Addr, opts...)
	defer client.Close()
	ws, resp, e := client.Websocket(context.Background(),
		BaseWebsocket+`/ws`,
		nil,
	)
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	if !assert.Equal(t, resp.Status, http.StatusSwitchingProtocols) {
		t.FailNow()
	}
	defer ws.Close()

	c, resp, e := client.Connect(context.Background(),
		"tcp://"+TCP,
	)
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	if !assert.Equal(t, resp.Status, http.StatusSwitchingProtocols) {
		t.FailNow()
	}
	defer c.Close()

	// TextMessage
	for i := 0; i < 20; i++ {
		str := fmt.Sprintf("str-%v", i)
		send := make([]byte, len(str)+2)
		binary.LittleEndian.PutUint16(send, uint16(len(str)))
		copy(send[2:], str)

		n, e := ws.WriteText(str)
		if !assert.Nil(t, e) {
			t.FailNow()
		}
		if !assert.Equal(t, len(str), n) {
			t.FailNow()
		}
		_, e = c.Write(send)
		if !assert.Nil(t, e) {
			t.FailNow()
		}

		ty, b, e := ws.ReadMessage()
		if !assert.Nil(t, e) {
			t.FailNow()
		}
		if !assert.Equal(t, websocket.TextMessage, ty) {
			t.FailNow()
		}
		if !assert.Equal(t, string(b), str) {
			t.FailNow()
		}

		recv := make([]byte, len(send))
		_, e = io.ReadFull(c, recv)
		if !assert.Nil(t, e) {
			t.FailNow()
		}
		if !assert.Equal(t, send, recv) {
			t.FailNow()
		}
	}
	// BinaryMessage
	for i := 0; i < 20; i++ {
		str := fmt.Sprintf("bytes-%v", i)
		n, e := ws.Write([]byte(str))
		if !assert.Nil(t, e) {
			t.FailNow()
		}
		if !assert.Equal(t, len(str), n) {
			t.FailNow()
		}
		ty, b, e := ws.ReadMessage()
		if !assert.Nil(t, e) {
			t.FailNow()
		}
		if !assert.Equal(t, websocket.BinaryMessage, ty) {
			t.FailNow()
		}
		if !assert.Equal(t, string(b), str) {
			t.FailNow()
		}
	}

	// get
	ch := make(chan struct {
		E    error
		Resp *httpadapter.MessageResponse
	})
	go func() {
		resp, e := client.Unary(context.Background(), &httpadapter.MessageRequest{
			URL:    BaseURL + "/text",
			Method: http.MethodGet,
		})
		ch <- struct {
			E    error
			Resp *httpadapter.MessageResponse
		}{
			E:    e,
			Resp: resp,
		}
	}()
	<-send
	// TextMessage
	for i := 0; i < 20; i++ {
		if !assert.Equal(t, atomic.LoadInt64(&step), int64(1)) {
			t.FailNow()
		}
		str := fmt.Sprintf("str-%v", i)

		send := make([]byte, len(str)+2)
		binary.LittleEndian.PutUint16(send, uint16(len(str)))
		copy(send[2:], str)

		n, e := ws.WriteText(str)
		if !assert.Nil(t, e) {
			t.FailNow()
		}
		if !assert.Equal(t, len(str), n) {
			t.FailNow()
		}
		_, e = c.Write(send)
		if !assert.Nil(t, e) {
			t.FailNow()
		}

		ty, b, e := ws.ReadMessage()
		if !assert.Nil(t, e) {
			t.FailNow()
		}
		if !assert.Equal(t, websocket.TextMessage, ty) {
			t.FailNow()
		}
		if !assert.Equal(t, string(b), str) {
			t.FailNow()
		}

		recv := make([]byte, len(send))
		_, e = io.ReadFull(c, recv)
		if !assert.Nil(t, e) {
			t.FailNow()
		}
		if !assert.Equal(t, send, recv) {
			t.FailNow()
		}
	}
	if !assert.Equal(t, atomic.LoadInt64(&step), int64(1)) {
		t.FailNow()
	}
	val := <-ch
	if !assert.Nil(t, val.E) {
		t.FailNow()
	}
	if !assert.Equal(t, http.StatusOK, val.Resp.Status) {
		t.FailNow()
	}
	b, e := io.ReadAll(val.Resp.Body)
	val.Resp.Body.Close()
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	if !assert.Equal(t, "ok", string(b)) {
		t.FailNow()
	}
	if !assert.Equal(t, atomic.LoadInt64(&step), int64(2)) {
		t.FailNow()
	}
}
