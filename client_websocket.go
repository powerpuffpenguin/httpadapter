package httpadapter

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/powerpuffpenguin/httpadapter/core"
)

// 請求代理訪問一個 websocket
func (c *Client) Websocket(ctx context.Context, u string, header http.Header) (ws *Websocket, resp *MessageResponse, e error) {
	uri, e := url.Parse(u)
	if e != nil {
		return
	} else if uri.Scheme != `ws` && uri.Scheme != `wss` || uri.Host == `` {
		e = errors.New(`not support url: ` + u)
		return
	}

	cc, resp, e := c.unary(ctx, nil, 0, &core.ClientMetadata{
		URL:    u,
		Header: header,
	})
	if e != nil {
		return
	} else if resp.Status != http.StatusSwitchingProtocols {
		defer cc.Close()
		var b []byte
		b, e = io.ReadAll(io.LimitReader(resp.Body, 256))
		if e == nil {
			if len(b) == 0 {
				e = errors.New(`switching protocols error`)
			} else {
				e = errors.New(string(b))
			}
		}
		return
	}
	ws = &Websocket{c: cc}
	return
}
