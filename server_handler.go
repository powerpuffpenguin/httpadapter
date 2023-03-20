package httpadapter

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/powerpuffpenguin/httpadapter/core"
)

type Handler interface {
	ServeChannel(c Conn)
}
type handlerFunc struct {
	f func(c Conn)
}

func HandleFunc(f func(c Conn)) Handler {
	return handlerFunc{
		f: f,
	}
}
func (h handlerFunc) ServeChannel(c Conn) {
	h.f(c)
}

var channelBytes = sync.Pool{
	New: func() any {
		return make([]byte, 1024)
	},
}
var defaultHandler channelHandler

type channelHandler struct {
}

func (h channelHandler) ServeChannel(c Conn) {
	f := &forwardConn{c: c}
	defer f.Close()
	f.Serve()
}

type forwardConn struct {
	c   Conn
	buf any
}

func (f *forwardConn) Close() {
	f.c.Close()
	f.freeBuffer()
}
func (f *forwardConn) freeBuffer() {
	if f.buf == nil {
		return
	}
	b := f.buf.([]byte)
	if len(b) == 1024 {
		channelBytes.Put(f.buf)
		f.buf = nil
	}
}
func (f *forwardConn) getBuffer(n int) (b []byte) {
	if f.buf == nil {
		if n > 1024 {
			return make([]byte, n)
		}
		f.buf = channelBytes.Get()
	}
	b = f.buf.([]byte)
	capacity := len(b)
	if capacity < n {
		channelBytes.Put(f.buf)

		b = make([]byte, n)
		f.buf = b
		return
	}
	b = b[:n]
	return
}
func (f *forwardConn) getBytes(end int) (buf *bytes.Buffer) {
	if f.buf == nil {
		f.buf = channelBytes.Get()
	}
	b := f.buf.([]byte)
	buf = bytes.NewBuffer(b[:end])
	return
}
func (f *forwardConn) Serve() {
	b := f.getBuffer(10)
	_, e := io.ReadFull(f.c, b)
	if e != nil {
		return
	}
	metalen := int(core.ByteOrder.Uint16(b))
	bodylen := core.ByteOrder.Uint64(b[2:])

	b = f.getBuffer(metalen)
	_, e = io.ReadFull(f.c, b)
	if e != nil {
		return
	}
	var metadata core.ClientMetadata
	err := metadata.Unmarshal(b)
	if err != nil {
		f.sendText(http.StatusBadRequest, err.Error())
		return
	}
	if bodylen > math.MaxInt64 {
		f.sendText(http.StatusBadRequest, "bodylen too large")
		return
	}
	// 驗證 url
	uri, err := url.Parse(metadata.URL)
	if err != nil {
		f.sendText(http.StatusBadRequest, err.Error())
		return
	}
	switch uri.Scheme {
	case "ws", "wss":
		f.websocket(&metadata, int64(bodylen))
	case "http", "https":
		switch metadata.Method {
		case http.MethodGet,
			http.MethodPost,
			http.MethodHead,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete:
			f.unary(&metadata, int64(bodylen))
		default:
			f.sendText(http.StatusBadRequest, `not support method: `+metadata.Method)
		}
	default:
		f.sendText(http.StatusBadRequest, `not support scheme: `+uri.Scheme)
	}
}
func (f *forwardConn) websocket(md *core.ClientMetadata, bodylen int64) {
	if bodylen != 0 {
		f.sendText(http.StatusBadRequest, `bodylen invalid`)
		return
	}
	ctx := f.c.Context()
	ws, _, e := websocket.DefaultDialer.DialContext(ctx, md.URL, md.Header)
	if e != nil {
		f.sendText(http.StatusBadGateway, e.Error())
		return
	}
	defer ws.Close()
	e = f.sendOk(http.StatusSwitchingProtocols, nil, nil, 0)
	if e != nil {
		return
	}

	go func() {
		var (
			b      = make([]byte, 1024*32)
			closed bool
			e      error
		)
		for {
			closed, e = f.readWS(ws, b)
			if e != nil {
				if closed {
					f.c.Close()
					time.Sleep(time.Second)
					ws.Close()
				} else {
					ws.Close()
					if f.c.Context().Err() == nil {
						time.Sleep(time.Second)
						f.c.Close()
					}
				}
				break
			}
		}
	}()
	var (
		b      = f.getBuffer(4)
		closed bool
	)
	for {
		closed, e = f.writeWS(ws, b)
		if e != nil {
			if closed {
				f.c.Close()
				time.Sleep(time.Second)
				ws.Close()
			} else {
				ws.Close()
				if f.c.Context().Err() == nil {
					time.Sleep(time.Second)
					f.c.Close()
				}
			}
			break
		}
	}
}
func (f *forwardConn) readWS(ws *websocket.Conn, b []byte) (closed bool, e error) {
	t, r, e := ws.NextReader()
	if e != nil {
		return
	}
	n, e := r.Read(b[4:])
	if e != nil {
		if e == io.EOF {
			e = nil
		}
		return
	}
	b[0] = byte(t)
	b[1] = 0
	core.ByteOrder.PutUint16(b[2:], uint16(n))
	_, e = f.c.Write(b[:4+n])
	if e != nil {
		closed = true
		return
	}
	for {
		n, e = r.Read(b[3:])
		if e != nil {
			if e == io.EOF {
				b[0] = 1
				core.ByteOrder.PutUint16(b[1:], uint16(n))
				_, e = f.c.Write(b[:3+n])
				if e != nil {
					closed = true
					return
				}
			}
			break
		}
		b[0] = 0
		core.ByteOrder.PutUint16(b[1:], uint16(n))
		_, e = f.c.Write(b[:3+n])
		if e != nil {
			closed = true
			return
		}
	}
	return
}
func (f *forwardConn) writeWS(ws *websocket.Conn, b []byte) (closed bool, e error) {
	_, e = io.ReadFull(f.c, b[:4])
	if e != nil {
		closed = true
		return
	}
	var (
		t   = b[0]
		end = b[1] == 1
		l   = core.ByteOrder.Uint16(b[2:])
	)
	w, e := ws.NextWriter(int(t))
	if e != nil {
		return
	}
	defer w.Close()
	if l != 0 {
		_, e = io.Copy(w, io.LimitReader(f.c, int64(l)))
		if e != nil {
			return
		}
	}
	for !end {
		_, e = io.ReadFull(f.c, b[:3])
		if e != nil {
			closed = true
			return
		}
		end = b[0] == 1
		l = core.ByteOrder.Uint16(b[1:])
		if l != 0 {
			_, e = io.Copy(w, io.LimitReader(f.c, int64(l)))
			if e != nil {
				return
			}
		}
	}
	e = w.Close()
	return
}
func (f *forwardConn) unary(md *core.ClientMetadata, bodylen int64) {
	// 創建 request
	ctx := f.c.Context()
	req, e := http.NewRequestWithContext(ctx, md.Method, md.URL, io.LimitReader(f.c, bodylen))
	if e != nil {
		f.sendText(http.StatusBadRequest, e.Error())
		return
	}
	// 設置 header
	for k, v := range md.Header {
		k0 := strings.ToLower(k)
		if k0 == `connection` ||
			k0 == `content-length` {
			continue
		}
		req.Header[k] = v
	}
	if bodylen > 0 {
		if req.Header.Get(`Content-Type`) == `` {
			req.Header.Set(`Content-Type`, `application/x-www-form-urlencoded`)
		}
	}
	// 發送請求
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		f.sendText(http.StatusBadGateway, e.Error())
		return
	}
	defer resp.Body.Close()

	// 解析數據
	s := resp.Header.Get(`content-length`)
	if s == `` {
		f.sendText(http.StatusBadGateway, `http response(`+strconv.Itoa(resp.StatusCode)+`) not set header: content-length`)
		return
	}
	length, e := strconv.ParseUint(s, 10, 64)
	if e != nil {
		f.sendText(http.StatusBadGateway, `content-length error: `+e.Error())
		return
	} else if length > math.MaxInt64 {
		f.sendText(http.StatusBadGateway, `content-length too large: `+strconv.FormatUint(length, 10))
		return
	}

	// 返回數據
	f.sendResponse(resp.StatusCode, resp.Header, resp.Body, uint64(length))
}
func (f *forwardConn) sendOk(status int, header http.Header, body io.Reader, bodylen uint64) (e error) {
	e = f.c.Context().Err()
	if e != nil {
		return
	}
	md := core.ServerMetadata{
		Status: status,
		Header: header,
	}

	w := f.getBytes(10)

	e = json.NewEncoder(w).Encode(md)
	if e != nil {
		Logger.Println(e)
		return
	}
	metalen := w.Len() - 10
	data := w.Bytes()
	core.ByteOrder.PutUint16(data, uint16(metalen))
	core.ByteOrder.PutUint64(data[2:], uint64(bodylen))
	_, e = f.c.Write(data)
	if e != nil {
		return
	}
	if bodylen != 0 {
		_, e = io.Copy(f.c, body)
	}
	return
}

func (f *forwardConn) sendResponse(status int, header http.Header, body io.Reader, bodylen uint64) {
	e := f.sendOk(status, header, body, bodylen)
	if e != nil {
		return
	}
	f.delayWait()
}
func (f *forwardConn) sendText(status int, body string) {
	if f.c.Context().Err() != nil {
		return
	}

	md := core.ServerMetadata{
		Status: status,
		Header: make(http.Header),
	}
	md.Header.Set(`Content-Type`, `text/plain; charset=utf-8`)
	w := f.getBytes(10)

	e := json.NewEncoder(w).Encode(md)
	if e != nil {
		Logger.Println(e)
		return
	}
	metalen := w.Len() - 10

	_, e = w.WriteString(body)
	if e != nil {
		Logger.Println(e)
		return
	}
	data := w.Bytes()
	core.ByteOrder.PutUint16(data, uint16(metalen))
	core.ByteOrder.PutUint64(data[2:], uint64(len(body)))
	_, e = f.c.Write(data)
	if e != nil {
		return
	}

	f.delayWait()
}

// 延遲一段時間，等待寫入數據被收到再關閉
func (f *forwardConn) delayWait() {
	timer := time.NewTimer(time.Second * 5)
	select {
	case <-timer.C:
	case <-f.c.Context().Done():
		if !timer.Stop() {
			<-timer.C
		}
	}
}
