package httpadapter

import (
	"bufio"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/powerpuffpenguin/httpadapter/core"
)

// 爲 客戶端/服務器 傳輸層提供一些通用功能
type baseTransport struct {
	// 結束信號
	done chan struct{}
	// 關閉標記
	closed int32
	// 網路連接
	c net.Conn
	// 對面窗口大小
	window uint32

	// 數據寫入通道
	ch chan []byte
}

// 關閉傳輸層 此後所有關聯的資源都應該關閉和釋放
func (t *baseTransport) Close() {
	if t.closed == 0 && atomic.SwapInt32(&t.closed, 1) == 0 {
		close(t.done)
		t.c.Close()
	}
}

var pingBuffer = []byte{byte(core.CommandPing)}

// 定時傳送 Ping 指令
func (t *baseTransport) servePing(active <-chan int, duration time.Duration) {
	var (
		at    = time.Now()
		timer = time.NewTimer(duration)
	)
	for {
		select {
		case <-t.done:
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
			wait := time.Since(at) - duration
			if wait > 0 {
				timer.Reset(wait)
			} else {
				select {
				case <-t.done:
					return
				case t.ch <- pingBuffer:
				}
				at = time.Now()
				timer.Reset(duration)
			}
		case <-active:
			at = time.Now()
		}
	}
}

// 合併數據並寫入到 tcp
func (t *baseTransport) serveWrite(active chan<- int, size int) {
	defer t.c.Close()
	var (
		b  []byte
		w  io.Writer = t.c
		wf *bufio.Writer
		e  error
	)
	if size > 0 {
		wf = bufio.NewWriterSize(w, size)
		w = wf
	}
	for {
		if active != nil {
			select {
			case active <- 2:
			default:
			}
		}

		// 讀取待寫入數據
		select {
		case b = <-t.ch:
			_, e = w.Write(b)
			if e != nil {
				return
			}
		case <-t.done:
			return
		}

		// 合併剩餘數據
	FM:
		for {
			select {
			case b = <-t.ch:
				_, e = w.Write(b)
				if e != nil {
					return
				}
			case <-t.done:
				return
			default:
				break FM
			}
		}
		// 刷新剩餘數據
		if wf != nil {
			e = wf.Flush()
			if e != nil {
				break
			}
		}
	}
}

// 響應 pong 指令
func (t *baseTransport) onPong(r io.Reader, buf []byte) (exit bool) {
	_, e := io.ReadFull(r, buf[1:5])
	if e != nil {
		exit = true
		return
	}
	b := make([]byte, 5)
	copy(b, buf[:5])
	select {
	case <-t.done:
		exit = true
		return
	case t.ch <- b:
		return
	default:
	}
	go func() {
		select {
		case <-t.done:
		case t.ch <- b:
		}
	}()
	return
}

// 發送一個 channel 關閉指令
func (t *baseTransport) sendClose(id uint64) {
	b := make([]byte, 1+8)
	b[0] = byte(core.CommandClose)
	core.ByteOrder.PutUint64(b[1:], id)
	select {
	case <-t.done:
	case t.ch <- b:
	}
}

// 返回數據寫入通達
func (t *clientTransport) getWriter() chan<- []byte {
	return t.ch
}

// 返回結束信號
func (t *clientTransport) Done() <-chan struct{} {
	return t.done
}
