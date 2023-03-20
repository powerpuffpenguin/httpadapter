package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/textproto"
	"net/url"

	"github.com/powerpuffpenguin/easygo"
	"github.com/powerpuffpenguin/httpadapter/core"
)

func (c *Client) unary(ctx context.Context, body io.Reader, bodylen uint64, md *core.ClientMetadata) (cc net.Conn, resp *MessageResponse, e error) {
	w := bytes.NewBuffer(make([]byte, 10, 256))
	e = json.NewEncoder(w).Encode(md)
	if e != nil {
		return
	}
	b := w.Bytes()
	metalen := len(b) - 10
	if metalen > math.MaxUint16 {
		e = errors.New(`metadata length too long`)
		return
	}
	core.ByteOrder.PutUint16(b, uint16(metalen))
	core.ByteOrder.PutUint64(b[2:], bodylen)
	conn, e := c.DialContext(ctx)
	if e != nil {
		return
	}
	ch := make(chan easygo.Pair[MessageResponse, error], 1)
	go func() {
		// write header md
		var e error
		var resp easygo.Pair[MessageResponse, error]
		_, e = conn.Write(b)
		if e != nil {
			resp.Second = e
			ch <- resp
			return
		}
		// write body
		if bodylen > 0 {
			_, e = io.Copy(conn, body)
			if e != nil {
				resp.Second = e
				ch <- resp
				return
			}
		}

		// read header
		data := b[:10]
		_, e = io.ReadFull(conn, data)
		if e != nil {
			resp.Second = e
			ch <- resp
			return
		}
		metalen := int(core.ByteOrder.Uint16(data))
		if metalen == 0 {
			resp.Second = errors.New(`metalen invalid`)
			ch <- resp
			return
		}
		bodylen := uint64(core.ByteOrder.Uint64(data[2:]))
		if bodylen > math.MaxInt64 {
			resp.Second = errors.New(`bodylen invalid`)
			ch <- resp
			return
		}

		// read md
		if cap(b) < metalen {
			data = make([]byte, metalen)
		} else {
			data = b[:metalen]
		}
		_, e = io.ReadFull(conn, data)
		if e != nil {
			resp.Second = e
			ch <- resp
			return
		}
		var md core.ServerMetadata
		e = json.Unmarshal(data, &md)
		if e != nil {
			resp.Second = e
			ch <- resp
			return
		}

		// read body
		if bodylen == 0 {
			resp.First.Body = nilReadCloser{}
		} else {
			resp.First.BodyLen = bodylen
			resp.First.Body = readCloser{
				Closer: conn,
				Reader: io.LimitReader(conn, int64(bodylen)),
			}
		}
		resp.First.Status = md.Status
		header := make(http.Header, len(md.Header))
		for k, v := range md.Header {
			header[textproto.CanonicalMIMEHeaderKey(k)] = v
		}
		resp.First.Header = header
		ch <- resp
	}()
	select {
	case <-ctx.Done():
		e = ctx.Err()
		conn.Close()
	case obj := <-ch:
		resp, e = &obj.First, obj.Second
		if e == nil {
			cc = conn
		} else {
			conn.Close()
		}
	}
	return
}

// 請求一個一元方法
func (c *Client) Unary(ctx context.Context, req *MessageRequest) (resp *MessageResponse, e error) {
	uri, e := url.Parse(req.URL)
	if e != nil {
		return
	} else if uri.Scheme != `http` && uri.Scheme != `https` || uri.Host == `` {
		e = errors.New(`not support url: ` + req.URL)
		return
	}

	var bodylen uint64
	switch req.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		if req.BodyLen != 0 && req.Body != nil {
			bodylen = req.BodyLen
		}
	case http.MethodGet, http.MethodHead, http.MethodDelete:
	default:
		e = errors.New(`not support method: ` + req.Method)
		return
	}
	if bodylen > math.MaxInt64 {
		e = errors.New(`body length too long`)
		return
	}
	cc, resp, e := c.unary(ctx, req.Body, req.BodyLen, &core.ClientMetadata{
		URL:    req.URL,
		Method: req.Method,
		Header: req.Header,
	})
	if e != nil {
		return
	} else if resp.Body == nil {
		cc.Close()
	}
	return
}

type readCloser struct {
	io.Reader
	io.Closer
}
type nilReadCloser struct{}

func (nilReadCloser) Close() error {
	return nil
}
func (nilReadCloser) Read(p []byte) (int, error) {
	return 0, io.EOF
}

type MessageRequest struct {
	// 請求的 http url
	URL string
	// 請求 方法
	Method string
	// 添加的 header
	Header http.Header
	// body 內容
	Body io.Reader
	// body 大小
	BodyLen uint64
}

type MessageResponse struct {
	// http 響應碼
	Status int
	// http 響應 heaer
	Header http.Header
	// 響應 body，調用者需要關閉它，無論是否有返回 body 它都不爲 nil
	Body io.ReadCloser
	// 響應 body 內容長度
	BodyLen uint64
}
