package httpadapter

import (
	"net"
	"time"

	"github.com/powerpuffpenguin/easygo/option"
	"github.com/powerpuffpenguin/httpadapter/internal/memory"
)

var defaultClientOptions = clientOptions{
	window:      32 * 1024,
	readBuffer:  4096,
	writeBuffer: 4096,
	dialer:      &net.Dialer{},
	allocator:   defaultAllocator,
}

type clientOptions struct {
	window uint32

	readBuffer  int
	writeBuffer int

	ping time.Duration

	dialer ClientDialer

	allocator memory.BufferAllocator
}
type ClientDialer interface {
	Dial(network, address string) (net.Conn, error)
}
type ClientOption option.Option[clientOptions]

// 設置客戶端 channel 窗口大小
func WithWindow(window uint32) ClientOption {
	return option.New(func(opts *clientOptions) {
		if window > 0 {
			opts.window = window
		}
	})
}

// 設置 tcp-chain 讀取緩衝區大小
func WithReadBuffer(readBuffer int) ClientOption {
	return option.New(func(opts *clientOptions) {
		opts.readBuffer = readBuffer
	})
}

// 設置 tcp-chain 寫入緩衝區大小
func WithWriteBuffer(writeBuffer int) ClientOption {
	return option.New(func(opts *clientOptions) {
		opts.writeBuffer = writeBuffer
	})
}

// 在 tcp-chain 上一段時間內如果沒有數據流動則發送一個 ping 指令驗證連接是否有效
//
// 如果時間小於 1s 則不會自動發送 ping
func WithPing(ping time.Duration) ClientOption {
	return option.New(func(opts *clientOptions) {
		opts.ping = ping
	})
}

// 設置如何連接到服務器
func WithDialer(dialer ClientDialer) ClientOption {
	return option.New(func(opts *clientOptions) {
		opts.dialer = dialer
	})
}

// 設置如何分配內存
func WithAllocator(allocator Allocator) ClientOption {
	return option.New(func(opts *clientOptions) {
		if allocator == nil {
			opts.allocator = defaultAllocator
		} else {
			opts.allocator = memory.BufferAllocator{Allocator: allocator}
		}
	})
}
