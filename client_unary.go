package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/textproto"

	"github.com/powerpuffpenguin/easygo"
	"github.com/powerpuffpenguin/httpadapter/core"
)

// 請求一個一元方法
func (c *Client) Unary(ctx context.Context, req *UnaryRequest) (resp *UnaryResponse, e error) {
	bodylen := 0
	switch req.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		if req.BodyLen != 0 && req.Body != nil {
			bodylen = req.BodyLen
		}
	}
	if bodylen > math.MaxUint32 {
		e = errors.New(`body length too long`)
		return
	}

	md := &core.ClientMetadata{
		URL:    req.URL,
		Method: req.Method,
		Header: req.Header,
	}
	w := bytes.NewBuffer(make([]byte, 6, 256))
	e = json.NewEncoder(w).Encode(md)
	if e != nil {
		return
	}
	b := w.Bytes()
	if len(b) > math.MaxUint16 {
		e = errors.New(`metadata length too long`)
		return
	}
	core.ByteOrder.PutUint16(b, uint16(len(b)-6))
	core.ByteOrder.PutUint32(b[2:], uint32(req.BodyLen))

	conn, e := c.DialContext(ctx)
	if e != nil {
		return
	}
	ch := make(chan easygo.Pair[UnaryResponse, error], 1)
	go func() {
		var resp easygo.Pair[UnaryResponse, error]
		_, e = conn.Write(b)
		if e != nil {
			resp.Second = e
			ch <- resp
			return
		}
		b := make([]byte, 6)
		_, e = io.ReadFull(conn, b)
		if e != nil {
			resp.Second = e
			ch <- resp
			return
		}
		metalen := int(core.ByteOrder.Uint16(b))
		if metalen == 0 {
			resp.Second = errors.New(`metalen invalid`)
			ch <- resp
			return
		}
		data := make([]byte, metalen)
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

		bodylen := int(core.ByteOrder.Uint32(b[2:]))
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
		conn.Close()
		e = ctx.Err()
	case obj := <-ch:
		resp, e = &obj.First, obj.Second
		if e != nil || resp.Body == nil {
			conn.Close()
		}
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

type UnaryRequest struct {
	// 請求的 http url
	URL string
	// 請求 方法
	Method string
	// 添加的 header
	Header http.Header
	// body 內容
	Body io.Reader
	// body 大小
	BodyLen int
}

type UnaryResponse struct {
	// http 響應碼
	Status int
	// http 響應 heaer
	Header http.Header
	// 響應 body，調用者需要關閉它，無論是否有返回 body 它都不爲 nil
	Body io.ReadCloser
	// 響應 body 內容長度
	BodyLen int
}
