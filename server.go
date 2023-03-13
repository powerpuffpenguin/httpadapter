package httpadapter

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/powerpuffpenguin/httpadapter/core"
	"github.com/powerpuffpenguin/httpadapter/pipe"
)

var ErrServerClosed = errors.New("httpadapter: Server closed")
var ErrChannelClosed = errors.New("httpadapter: Channel closed")
var ErrTCPClosed = errors.New("httpadapter: TCp closed")

// httpadapter 服務器
type Server struct {
	opts   serverOptions
	done   chan struct{}
	closed int32
}

// 創建一個 適配 服務器
func NewServer(opt ...ServerOption) *Server {
	var opts = defaultServerOptions
	for _, o := range opt {
		o.Apply(&opts)
	}
	return &Server{
		opts: opts,
		done: make(chan struct{}),
	}
}

// 關閉服務
func (s *Server) Close() (e error) {
	if s.closed == 0 && atomic.SwapInt32(&s.closed, 1) == 0 {
		close(s.done)
	} else {
		e = ErrServerClosed
	}
	return
}

// 監聽並運行
func (s *Server) ListenAndServe(addr string) (e error) {
	l, e := net.Listen(`tcp`, addr)
	if e != nil {
		return
	}
	e = s.Serve(l)
	return
}

// 監聽 tls 並運行
func (s *Server) ListenAndServeTLS(addr, certFile, keyFile string) (e error) {
	l, e := net.Listen(`tcp`, addr)
	if e != nil {
		return
	}
	var config tls.Config
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], e = tls.LoadX509KeyPair(certFile, keyFile)
	if e != nil {
		return
	}

	tlsListener := tls.NewListener(l, &config)
	e = s.Serve(tlsListener)
	return
}

// 在監聽器上運行
func (s *Server) Serve(l net.Listener) (e error) {
	var wait sync.WaitGroup
	var hl *httpListner
	if s.opts.handler != nil {
		hl = &httpListner{
			done: s.done,
			ch:   make(chan net.Conn),
		}
		wait.Add(1)
		go func() {
			http.Serve(hl, s.opts.handler)
			wait.Done()
		}()
	}
	wait.Add(1)
	go func() {
		<-s.done
		l.Close()
		wait.Done()
	}()
	var tempDelay time.Duration // how long to sleep on accept failure
SS:
	for {
		rw, err := l.Accept()
		if err != nil {
			select {
			case <-s.done:
				e = ErrServerClosed
				break SS
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				ServerLogger.Printf("httpadapter: Accept error: %v; retrying in %v\n", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			e = err
			break
		}
		tempDelay = 0
		go s.serve(hl, rw)
	}

	wait.Wait()
	return
}

type asyncHello struct {
	backend net.Conn
	code    core.Hello
	version string
	window  uint16
	e       error
}

func (s *Server) serve(l *httpListner, rw net.Conn) {
	var (
		backend net.Conn
		e       error
		b       = make([]byte, 256)
		code    core.Hello
		version string
		window  uint16
	)
	if s.opts.timeout > 0 {
		timer := time.NewTimer(s.opts.timeout)
		ch := make(chan *asyncHello, 1)
		go func() {
			backend, code, version, window, e := s.hello(rw, b)
			obj := &asyncHello{
				backend: backend,
				code:    code,
				version: version,
				window:  window,
				e:       e,
			}
			ch <- obj
		}()
		select {
		case <-timer.C:
			e = context.DeadlineExceeded
		case obj := <-ch:
			if !timer.Stop() {
				<-timer.C
			}
			backend, code, version, window, e = obj.backend, obj.code, obj.version, obj.window, obj.e
		}
	} else {
		backend, code, version, window, e = s.hello(rw, b)
	}
	// hello 錯誤
	if e != nil {
		rw.Close()
		return
	}

	if backend != nil {
		// 轉發到後端
		if s.opts.backend != nil {
			dst, e := s.opts.backend.Dial()
			if e == nil {
				go pipe.Copy(dst, backend, nil)
				pipe.Copy(backend, dst, nil)
			} else {
				backend.Close()
			}
			return
		}
		// 兼容 http
		if l != nil {
			select {
			case l.ch <- backend:
				return
			case <-l.done:
				backend.Close()
				return
			}
		}
		// 返回協議未知
		e = s.sendHello(rw, b, core.HelloInvalidProtocol, ``)
		if e != nil {
			rw.Close()
			return
		}
		time.Sleep(time.Second)
		rw.Close()
		return
	}
	// 連接成功
	e = s.sendHello(rw, b, code, version)
	if e != nil || code != 0 {
		rw.Close()
		return
	}

	// 執行轉發
	newServerTransport(rw,
		int(s.opts.window), int(window),
		s.opts.channelHandler,
	).Serve(
		b,
		s.opts.readBuffer, s.opts.writeBuffer,
		s.opts.channels,
		s.opts.ping,
	)
}
func (s *Server) sendHello(rw net.Conn, b []byte, hello core.Hello, version string) (e error) {
	msg := core.ServerHello{
		Code:   hello,
		Window: s.opts.window,
	}
	if hello == core.HelloOk {
		msg.Message = version
	} else {
		msg.Message = hello.String()
	}
	data, e := msg.MarshalTo(b)
	if e != nil {
		return
	}
	_, e = rw.Write(data)
	return
}
func (s *Server) hello(rw net.Conn, b []byte) (backend net.Conn, code core.Hello, version string, window uint16, e error) {
	msg, code, flag, e := core.ReadClientHello(rw, b)
	if code == core.HelloInvalidProtocol {
		backend = &httpConn{
			Conn: rw,
			b:    flag,
		}
		return
	}
	if e != nil {
		if e == io.ErrShortBuffer {
			e = nil
			code = core.HelloInvalidVersion
		}
		return
	} else if code != core.HelloOk {
		return
	}
	for _, v := range msg.Version {
		if v == core.ProtocolVersion {
			code = core.HelloOk
			window = msg.Window
			version = core.ProtocolVersion
			return
		}
	}
	code = core.HelloInvalidVersion
	return
}

// 返回服務器的 channel window 大小
func (s *Server) Window() uint16 {
	return s.opts.window
}

// 返回服務器兼容的 http 處理器
func (s *Server) HTTP() http.Handler {
	return s.opts.handler
}

// 返回服務器的 channel 處理器
func (s *Server) Handler() Handler {
	return s.opts.channelHandler
}

// 返回未知協議的後端
func (s *Server) Backend() Backend {
	return s.opts.backend
}

// 返回客戶端連接的超時時間，<1 則永不超時
func (s *Server) Timeout() time.Duration {
	return s.opts.timeout
}

// 返回 tcp-chain 讀取緩衝區大小
func (s *Server) ReadBuffer() int {
	return s.opts.readBuffer
}

// 返回 tcp-chain  寫入緩衝區大小
func (s *Server) WriteBuffer() int {
	return s.opts.writeBuffer
}

// 返回服務器運行的 併發 channel 數量
func (s *Server) Channels() int {
	return s.opts.channels
}

// 返回服務器每隔多久對沒有數據的連接發送 tcp ping，<1 則不會發送
func (s *Server) Ping() time.Duration {
	return s.opts.ping
}
