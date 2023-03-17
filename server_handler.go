package httpadapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

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

var defaultHandler = channelHandler{}
var channelBytes = sync.Pool{
	New: func() any {
		return make([]byte, 1024)
	},
}

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
	next := f.readFirst()
	if !next {
		return
	}
	fmt.Println("websocket")
}
func (f *forwardConn) readFirst() (next bool) {
	b := f.getBuffer(6)
	_, e := io.ReadFull(f.c, b)
	if e != nil {
		return
	}
	metalen := int(core.ByteOrder.Uint16(b))
	bodylen := int(core.ByteOrder.Uint32(b[2:]))

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
	// 驗證 url
	_, err = url.Parse(metadata.URL)
	if err != nil {
		f.sendText(http.StatusBadRequest, err.Error())
		return
	}
	// 驗證
	switch metadata.Method {
	case http.MethodGet,
		http.MethodPost,
		http.MethodHead,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete:
		f.unary(&metadata, bodylen)
	default:
		f.sendText(http.StatusBadRequest, `not support method`)
	}
	return
}
func (f *forwardConn) unary(md *core.ClientMetadata, bodylen int) {
	// 創建 request
	ctx := f.c.Context()
	req, e := http.NewRequestWithContext(ctx, md.Method, md.URL, io.LimitReader(f.c, int64(bodylen)))
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
	} else if length > math.MaxUint32 || length > math.MaxInt {
		f.sendText(http.StatusBadGateway, `content-length too large: `+strconv.FormatUint(length, 10))
		return
	}

	// 返回數據
	f.sendResponse(resp.StatusCode, resp.Header, resp.Body, uint32(length))
}
func (f *forwardConn) sendResponse(status int, header http.Header, body io.Reader, bodylen uint32) {
	if f.c.Context().Err() != nil {
		return
	}
	md := core.ServerMetadata{
		Status: status,
		Header: header,
	}

	w := f.getBytes(6)

	e := json.NewEncoder(w).Encode(md)
	if e != nil {
		Logger.Println(e)
		return
	}
	metalen := w.Len() - 6
	data := w.Bytes()
	core.ByteOrder.PutUint16(data, uint16(metalen))
	core.ByteOrder.PutUint32(data[2:], uint32(bodylen))
	_, e = f.c.Write(data)
	if e != nil {
		return
	}
	_, e = io.Copy(f.c, body)
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
	w := f.getBytes(6)

	e := json.NewEncoder(w).Encode(md)
	if e != nil {
		Logger.Println(e)
		return
	}
	metalen := w.Len() - 6

	_, e = w.WriteString(body)
	if e != nil {
		Logger.Println(e)
		return
	}
	data := w.Bytes()
	core.ByteOrder.PutUint16(data, uint16(metalen))
	core.ByteOrder.PutUint32(data[2:], uint32(len(body)))

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
