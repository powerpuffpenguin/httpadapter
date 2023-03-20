package httpadapter

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/powerpuffpenguin/httpadapter/core"
)

var errWriterClosed = errors.New(`writer closed`)

type Websocket struct {
	c net.Conn
}

// return writer not goroutine safe
func (w *Websocket) NextWriter(t int) (io.WriteCloser, error) {
	switch t {
	case websocket.TextMessage, websocket.BinaryMessage,
		websocket.CloseMessage, websocket.PingMessage, websocket.PongMessage:
	default:
		return nil, errors.New(`unknow websocekt message type: ` + strconv.Itoa(t))
	}
	return &websocketWriter{
		t: byte(t),
		b: make([]byte, 4),
		w: w.c,
	}, nil
}

// not goroutine safe
func (w *Websocket) WriteMessage(t int, b []byte) (n int, e error) {
	dst, e := w.NextWriter(t)
	if e != nil {
		return
	}
	n, e = dst.Write(b)
	if e != nil {
		return
	}
	e = dst.Close()
	return
}

// not goroutine safe
func (w *Websocket) WriteText(s string) (n int, e error) {
	dst, e := w.NextWriter(websocket.TextMessage)
	if e != nil {
		return
	}
	n, e = dst.Write(core.StringToBytes(s))
	if e != nil {
		return
	}
	e = dst.Close()
	return
}

// not goroutine safe
func (w *Websocket) Write(b []byte) (n int, e error) {
	dst, e := w.NextWriter(websocket.BinaryMessage)
	if e != nil {
		return
	}
	n, e = dst.Write(b)
	if e != nil {
		return
	}
	e = dst.Close()
	return
}

// not goroutine safe
func (w *Websocket) WriteJSON(obj any) (e error) {
	b, e := json.Marshal(obj)
	if e != nil {
		return
	}
	_, e = w.WriteMessage(websocket.TextMessage, b)
	return
}

// return reader not goroutine safe
func (w *Websocket) NextReader() (t int, r io.Reader, e error) {
	b := make([]byte, 4)
	_, e = io.ReadFull(w.c, b)
	if e != nil {
		return
	}
	t = int(b[0])
	size := core.ByteOrder.Uint16(b[2:])
	r = &websocketReader{
		c: w.c,
		b: b[1:4],
		r: io.LimitReader(w.c, int64(size)),
	}
	return
}

// not goroutine safe
func (w *Websocket) ReadMessage() (t int, b []byte, e error) {
	t, r, e := w.NextReader()
	if e != nil {
		return
	}
	b, e = io.ReadAll(r)
	return
}

type websocketReader struct {
	c   io.Reader
	r   io.Reader
	b   []byte
	err error
	sync.Mutex
}

func (r *websocketReader) Read(b []byte) (n int, e error) {
	r.Lock()
	defer r.Unlock()
	if r.err != nil {
		return
	}
	for r.r == nil {
		_, e = io.ReadFull(r.c, r.b)
		if e != nil {
			r.err = e
			return
		}
		size := core.ByteOrder.Uint16(r.b[1:])
		if size != 0 {
			r.r = io.LimitReader(r.c, int64(size))
			break
		}

		if r.b[0] != 0 {
			e = io.EOF
			r.err = e
			return
		}
	}
	n, e = r.r.Read(b)
	if e == nil {
		return
	} else if e != io.EOF || r.b[0] != 0 {
		r.err = e
		return
	}
	e = nil
	r.r = nil
	return
}

type websocketWriter struct {
	t byte
	b []byte
	w io.Writer
	sync.Mutex
	closed  bool
	writerd bool
	err     error
}

func (w *websocketWriter) Close() (e error) {
	w.Lock()
	if w.closed {
		e = errWriterClosed
		w.err = e
	} else {
		w.closed = true
		data := w.b[:3]
		data[0] = 1
		core.ByteOrder.PutUint16(data[1:], 0)
		w.w.Write(data)
	}
	w.Unlock()
	return
}

func (w *websocketWriter) Write(b []byte) (n int, e error) {
	w.Lock()
	defer w.Unlock()
	if w.err != nil {
		e = w.err
		return
	}

	if !w.writerd {
		data := w.b[:1]
		data[0] = w.t
		_, e = w.w.Write(data)
		if e != nil {
			w.err = e
			return
		}
		w.writerd = true
	}
	var (
		data = w.b[:3]
		size int
	)
	for {
		size = len(b)
		if size == 0 {
			break
		}

		if size > math.MaxUint16 {
			size = math.MaxUint16
		}
		data[0] = 0
		core.ByteOrder.PutUint16(data[1:], uint16(size))
		_, e = w.w.Write(data)
		if e != nil {
			w.err = e
			return
		}

		_, e = w.w.Write(b[:size])
		if e != nil {
			w.err = e
			return
		}
		n += size
		b = b[size:]
	}
	return
}
