package httpadapter

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
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
func NewServer(opt ...Option[serverOptions]) *Server {
	var opts = defaultServerOptions
	for _, o := range opt {
		o.apply(&opts)
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
func (s *Server) Serve(l net.Listener) error {
	var hl *httpListner
	if s.opts.handler != nil {
		hl = &httpListner{
			done: s.done,
			ch:   make(chan net.Conn),
		}
		go http.Serve(hl, s.opts.handler)
	}
	go func() {
		<-s.done
		l.Close()
	}()
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		rw, err := l.Accept()
		if err != nil {
			select {
			case <-s.done:
				return ErrServerClosed
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
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
			return err
		}
		tempDelay = 0
		go s.serve(hl, rw)
	}
}

type asyncHello struct {
	backend net.Conn
	code    Hello
	version string
	window  uint16
	e       error
}

func (s *Server) serve(l *httpListner, rw net.Conn) {
	var (
		backend net.Conn
		e       error
		b       = make([]byte, 1024)
		code    Hello
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
				go Copy(dst, backend, nil)
				Copy(backend, dst, nil)
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
		e = s.sendHello(rw, b, HelloInvalidProtocol, ``)
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
func (s *Server) sendHello(rw net.Conn, b []byte, hello Hello, version string) (e error) {
	copy(b, StringToBytes(Flag))

	i := len(Flag)
	b[i] = uint8(hello)
	i++

	if hello == HelloOk {
		binary.BigEndian.PutUint16(b[i:], s.opts.window)
	} else {
		binary.BigEndian.PutUint16(b[i:], 0)
	}
	i += 2

	if hello == HelloOk {
		binary.BigEndian.PutUint16(b[i:], uint16(len(version)))
		i += 2

		n := copy(b[i:], version)
		i += n
		_, e = rw.Write(b[:i])
	} else {
		msg := hello.String()
		binary.BigEndian.PutUint16(b[i:], uint16(len(msg)))
		i += 2

		n := copy(b[i:], msg)
		i += n
		_, e = rw.Write(b[:i])
	}
	return
}
func (s *Server) hello(rw net.Conn, b []byte) (backend net.Conn, code Hello, version string, window uint16, e error) {
	min := len(Flag)
	_, e = io.ReadFull(rw, b[:min])
	if e != nil {
		return
	}
	if BytesToString(b[:min]) != Flag {
		backend = &httpConn{
			Conn: rw,
			b:    b[:min],
		}
		return
	}
	min = 4
	_, e = io.ReadFull(rw, b[:min])
	if e != nil {
		return
	}
	window = binary.BigEndian.Uint16(b)
	if window < 1 {
		code = HelloWindow
		return
	}
	min = int(binary.BigEndian.Uint16(b[2:]))
	if len(b) < min {
		b = make([]byte, min)
	} else {
		b = b[:min]
	}
	_, e = io.ReadFull(rw, b)
	if e != nil {
		return
	}
	str := BytesToString(b)
	strs := strings.Split(str, ",")
	for _, str := range strs {
		if str == ProtocolVersion {
			code = HelloOk
			version = ProtocolVersion
			return
		}
	}
	code = HelloInvalidVersion
	return
}

// 返回服務器的 channel window 大小
func (s *Server) Window() uint16 {
	return s.opts.window
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
