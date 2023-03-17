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

// 請求代理訪問一個 websocket
func (c *Client) Websocket(ctx context.Context, url string, header http.Header) (ws *Websocket, resp *MessageResponse, e error) {
	md := &core.ClientMetadata{
		URL:    url,
		Header: header,
	}
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
	core.ByteOrder.PutUint64(b[2:], 0)
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
		if md.Status != http.StatusSwitchingProtocols {
			b, e := io.ReadAll(io.LimitReader(resp.First.Body, 256))
			if e == nil {
				if len(b) == 0 {
					resp.Second = errors.New(`switching protocols error`)
				} else {
					resp.Second = errors.New(string(b))
				}
				ch <- resp
			} else {
				resp.Second = e
				ch <- resp
			}
			return
		}
		ch <- resp
	}()
	select {
	case <-ctx.Done():
		conn.Close()
		e = ctx.Err()
	case obj := <-ch:
		resp, e = &obj.First, obj.Second
		if e != nil {
			conn.Close()
			return
		}
		ws = &Websocket{c: conn}
	}
	return
}
