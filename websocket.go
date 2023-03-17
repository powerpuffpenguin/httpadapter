package httpadapter

import (
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
	if w.closed {
		e = errWriterClosed
		return
	} else if w.err != nil {
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
