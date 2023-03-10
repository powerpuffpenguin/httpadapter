package httpadapter

import (
	"net"
	"net/http"
	"time"
)

var defaultServerOptions = serverOptions{
	window:      32 * 1024,
	timeout:     time.Second * 10,
	readBuffer:  4096,
	writeBuffer: 4096,
	channels:    0,
}

type serverOptions struct {
	window      uint16
	timeout     time.Duration
	handler     http.Handler
	backend     Backend
	readBuffer  int
	writeBuffer int
	channels    int
}
type Backend interface {
	Dial() (net.Conn, error)
}

// 設置服務器 channel 窗口大小
func ServerWindow(window uint16) Option[serverOptions] {
	return NewOption(func(opts *serverOptions) {
		if window > 0 {
			opts.window = window
		}
	})
}

// 如果設置了 http.Handler，將在服務器同一端口上共享 http 服務 和 httpadapter 服務
func ServerHTTP(handler http.Handler) Option[serverOptions] {
	return NewOption(func(opts *serverOptions) {
		opts.handler = handler
	})
}

// 如果 backend 不爲空字符串，則將 httpadapter 之外的協議轉發到此後端
func ServerBackend(backend Backend) Option[serverOptions] {
	return NewOption(func(opts *serverOptions) {
		opts.backend = backend
	})
}
func NewTCPBackend(addr string) Backend {
	return tcpBackend{
		addr: addr,
	}
}

type tcpBackend struct {
	addr string
}

func (b tcpBackend) Dial() (net.Conn, error) {
	return net.Dial(`tcp`, b.addr)
}

// 如果在 tcp-chain 連接成功後經過了 timeout 指定的時間還未收到 hello 消息則斷開 tcp
func ServerTimeout(timeout time.Duration) Option[serverOptions] {
	return NewOption(func(opts *serverOptions) {
		opts.timeout = timeout
	})
}

// 設置 tcp-chain 讀取緩衝區大小
func ServerReadBuffer(readBuffer int) Option[serverOptions] {
	return NewOption(func(opts *serverOptions) {
		opts.readBuffer = readBuffer
	})
}

// 設置 tcp-chain 寫入緩衝區大小
func ServerWriteBuffer(writeBuffer int) Option[serverOptions] {
	return NewOption(func(opts *serverOptions) {
		opts.writeBuffer = writeBuffer
	})
}

// 設置允許併發存在的 channel 數量，如果 < 1 則不進行限制
func ServerWriteBufferChannels(channels int) Option[serverOptions] {
	return NewOption(func(opts *serverOptions) {
		opts.channels = channels
	})
}
