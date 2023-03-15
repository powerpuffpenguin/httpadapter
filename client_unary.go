package httpadapter

import (
	"context"
	"io"
	"net/http"
)

// 請求一個一元方法
func (c *Client) Unary(ctx context.Context, req *UnaryRequest) (resp *UnaryResponse, e error) {
	// conn, e := c.DialContext(ctx)
	// if e != nil {
	// 	return
	// }

	return
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
}
