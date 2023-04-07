package httpadapter

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/powerpuffpenguin/easygo/option"
)

var defaultServerOptions = serverOptions{
	window:         32 * 1024,
	timeout:        time.Second * 10,
	readBuffer:     4096,
	writeBuffer:    4096,
	channels:       0,
	channelHandler: defaultHandler,
	tcpDialer:      DefaultTCPDialer{},
}

type serverOptions struct {
	window         uint32
	timeout        time.Duration
	handler        http.Handler
	backend        Backend
	readBuffer     int
	writeBuffer    int
	channels       int
	channelHandler Handler
	ping           time.Duration
	tcpDialer      TCPDialer
	hookURL        HookURL
	hookDo         HookDo
}
type HookDo interface {
	Do(req *http.Request) (*http.Response, error)
}
type hookDoFunc struct {
	f func(req *http.Request) (*http.Response, error)
}

func HookDoFunc(f func(req *http.Request) (resp *http.Response, e error)) HookDo {
	return hookDoFunc{
		f: f,
	}
}
func (h hookDoFunc) Do(req *http.Request) (*http.Response, error) {
	return h.f(req)
}

type HookURL interface {
	Hook(url *url.URL) (*url.URL, error)
}

type hookURLFunc struct {
	f func(*url.URL) (*url.URL, error)
}

func HookURLFunc(f func(*url.URL) (*url.URL, error)) HookURL {
	return hookURLFunc{
		f: f,
	}
}
func (h hookURLFunc) Hook(u *url.URL) (*url.URL, error) {
	return h.f(u)
}

type tcpTCPDialerFunc struct {
	f func(ctx context.Context, addr string, tls bool) (net.Conn, error)
}

func (d tcpTCPDialerFunc) DialContext(ctx context.Context, addr string, tls bool) (net.Conn, error) {
	return d.f(ctx, addr, tls)
}
func TCPDialerFunc(f func(ctx context.Context, addr string, tls bool) (c net.Conn, e error)) TCPDialer {
	return tcpTCPDialerFunc{
		f: f,
	}
}

type TCPDialer interface {
	DialContext(ctx context.Context, addr string, tls bool) (net.Conn, error)
}
type DefaultTCPDialer struct{}

func (DefaultTCPDialer) DialContext(ctx context.Context, addr string, safe bool) (net.Conn, error) {
	if safe {
		var dialer tls.Dialer
		return dialer.DialContext(ctx, `tcp`, addr)
	} else {
		var dialer net.Dialer
		return dialer.DialContext(ctx, `tcp`, addr)
	}
}

type Backend interface {
	Dial() (net.Conn, error)
}
type ServerOption = option.Option[serverOptions]

// 設置服務器 channel 窗口大小
func ServerWindow(window uint32) ServerOption {
	return option.New(func(opts *serverOptions) {
		if window > 0 {
			opts.window = window
		}
	})
}

// 如果設置了 http.Handler，將在服務器同一端口上共享 http 服務 和 httpadapter 服務
func ServerHTTP(handler http.Handler) ServerOption {
	return option.New(func(opts *serverOptions) {
		opts.handler = handler
	})
}

// 設置如何處理 channel
func ServerHandler(handler Handler) ServerOption {
	return option.New(func(opts *serverOptions) {
		if handler == nil {
			opts.channelHandler = defaultHandler
		} else {
			opts.channelHandler = handler
		}
	})
}

// 如果 backend 不爲空字符串，則將 httpadapter 之外的協議轉發到此後端
func ServerBackend(backend Backend) ServerOption {
	return option.New(func(opts *serverOptions) {
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
func ServerTimeout(timeout time.Duration) ServerOption {
	return option.New(func(opts *serverOptions) {
		opts.timeout = timeout
	})
}

// 設置 tcp-chain 讀取緩衝區大小
func ServerReadBuffer(readBuffer int) ServerOption {
	return option.New(func(opts *serverOptions) {
		opts.readBuffer = readBuffer
	})
}

// 設置 tcp-chain 寫入緩衝區大小
func ServerWriteBuffer(writeBuffer int) ServerOption {
	return option.New(func(opts *serverOptions) {
		opts.writeBuffer = writeBuffer
	})
}

// 設置允許併發存在的 channel 數量，如果 < 1 則不進行限制
func ServerWriteBufferChannels(channels int) ServerOption {
	return option.New(func(opts *serverOptions) {
		opts.channels = channels
	})
}

// 在 tcp-chain 上一段時間內如果沒有數據流動則發送一個 ping 指令驗證連接是否有效
//
// 如果時間小於 1s 則不會自動發送 ping
func ServerPing(ping time.Duration) ServerOption {
	return option.New(func(opts *serverOptions) {
		opts.ping = ping
	})
}

// 設置服務器在單個 tcp-chain 上允許的最大併發 channel 數量，如果 < 1 則不限制
func ServerChannels(channels int) ServerOption {
	return option.New(func(opts *serverOptions) {
		opts.channels = channels
	})
}

// 設置服務器如何連接轉發的 tcp
func ServerTCPDialer(dialer TCPDialer) ServerOption {
	return option.New(func(opts *serverOptions) {
		if dialer == nil {
			opts.tcpDialer = DefaultTCPDialer{}
		} else {
			opts.tcpDialer = dialer
		}
	})
}

// 設置一個 hook 用於在轉發前對 目標 url 進行 過濾
func ServerHookURL(h HookURL) ServerOption {
	return option.New(func(opts *serverOptions) {
		opts.hookURL = h
	})
}

// 設置一個 hook 用於自定義如何發送 http 請求
func ServerHookDo(h HookDo) ServerOption {
	return option.New(func(opts *serverOptions) {
		opts.hookDo = h
	})
}
