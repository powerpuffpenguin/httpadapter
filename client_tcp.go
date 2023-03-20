package httpadapter

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/powerpuffpenguin/httpadapter/core"
)

// 請求代理訪問一個 tcp/tls
func (c *Client) Connect(ctx context.Context, u string) (tc net.Conn, resp *MessageResponse, e error) {
	uri, e := url.Parse(u)
	if e != nil {
		return
	} else if uri.Scheme != `tcp` && uri.Scheme != `tls` || uri.Host == `` || uri.Port() == `` {
		e = errors.New(`not support url: ` + u)
		return
	}
	cc, resp, e := c.unary(ctx, nil, 0, &core.ClientMetadata{
		URL: u,
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
	tc = cc
	return
}
