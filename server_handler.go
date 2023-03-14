package httpadapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/powerpuffpenguin/httpadapter/core"
)

type Handler interface {
	ServeChannel(c net.Conn)
}
type handlerFunc struct {
	f func(c net.Conn)
}

func HandleFunc(f func(c net.Conn)) Handler {
	return handlerFunc{
		f: f,
	}
}
func (h handlerFunc) ServeChannel(c net.Conn) {
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

func (h channelHandler) ServeChannel(c net.Conn) {
	f := &forwardConn{c: c}
	defer f.Close()
	f.Serve()

	// defer c.Close()

	// buf := make([]byte, 1024)
	// _, e := io.ReadFull(c, buf)
	// if e != nil {
	// 	return
	// }
	// metalen := int(core.ByteOrder.Uint16(buf))
	// bodylen := core.ByteOrder.Uint32(buf[2:])

	// if len(buf) < metalen {
	// 	buf = make([]byte, metalen)
	// }
	// _, e = io.ReadFull(c, buf[:metalen])
	// if e != nil {
	// 	return
	// }
	// var msg struct {
	// 	URL    string              `json:"url"`
	// 	Method string              `json:"method"`
	// 	Header map[string][]string `json:"header"`
	// }
	// e = json.Unmarshal(buf[:metalen], &msg)
	// if e != nil {
	// 	h.sendText(c, buf,
	// 		core.ServerMetadata{
	// 			Status: http.StatusBadRequest,
	// 		},
	// 		core.StringToBytes(e.Error()),
	// 	)
	// 	return
	// }

	// req, e := http.NewRequest(msg.Method, msg.URL, io.LimitReader(c, int64(bodylen)))
	// if e != nil {
	// 	h.sendText(c, buf,
	// 		core.ServerMetadata{
	// 			Status: http.StatusBadRequest,
	// 		},
	// 		core.StringToBytes(e.Error()),
	// 	)
	// 	return
	// }
	// http.DefaultClient.Do(req)
}

type forwardConn struct {
	c   net.Conn
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
	_, e := f.readFirst()
	if e != nil {
		return
	}
	fmt.Println(f)
}
func (f *forwardConn) readFirst() (bodylen int, e error) {
	b := f.getBuffer(6)
	_, e = io.ReadFull(f.c, b)
	if e != nil {
		return
	}
	metalen := int(core.ByteOrder.Uint16(b))
	bodylen = int(core.ByteOrder.Uint32(b[2:]))

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
	case http.MethodGet:
		fallthrough
	case http.MethodHead:
		fallthrough
	case http.MethodPost:
		fallthrough
	case http.MethodPut:
		fallthrough
	case http.MethodPatch:
		fallthrough
	case http.MethodDelete:
		fmt.Println(metadata)
		return
	default:
		f.sendText(http.StatusBadRequest, `not support method`)
		return
	}

}

func (f *forwardConn) sendText(status int, body string) {
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
	time.Sleep(time.Second)
}
