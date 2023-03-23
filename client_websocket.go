package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/powerpuffpenguin/httpadapter/core"
	"github.com/powerpuffpenguin/httpadapter/internal/pipe"
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
		buffer := make([]byte, 256)
		var n int
		n, e = pipe.ReadAll(resp.Body, buffer)
		if e == nil {
			if n == 0 {
				e = errors.New(`switching protocols error`)
			} else {
				e = errors.New(string(buffer[:n]))
			}
		}
		return
	}
	ws = &Websocket{c: cc}
	return
}
