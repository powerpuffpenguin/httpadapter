package core

import (
	"bufio"
	"io"
	"time"
)

// 定時發送 ping
func Ping(
	duration time.Duration,
	done <-chan struct{},
	active <-chan int,
	w chan<- []byte,
	data []byte,
) {
	var (
		at    = time.Now()
		timer = time.NewTimer(duration)
	)
	for {
		select {
		case <-done:
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
				case <-done:
					return
				case w <- data:
				}
				at = time.Now()
				timer.Reset(duration)
			}
		case <-active:
			at = time.Now()
		}
	}
}

// 合併數據寫入
func Write(wc io.WriteCloser, size int,
	done <-chan struct{},
	active chan<- int,
	r <-chan []byte,
) {
	defer wc.Close()
	var (
		b  []byte
		w  io.Writer = wc
		wf *bufio.Writer
		e  error
	)
	if size > 0 {
		wf = bufio.NewWriterSize(wc, size)
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
		case b = <-r:
			_, e = w.Write(b)
			if e != nil {
				return
			}
		case <-done:
			return
		}

		// 合併剩餘數據
	FM:
		for {
			select {
			case b = <-r:
				_, e = w.Write(b)
				if e != nil {
					return
				}
			case <-done:
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

// 投遞數據到 chan
func PostWrite(done <-chan struct{},
	w chan<- []byte,
	b []byte,
) (exit bool,
) {
	select {
	case <-done:
		exit = true
		return
	case w <- b:
		return
	default:
	}
	go func() {
		select {
		case <-done:
			return
		case w <- b:
			return
		}
	}()
	return
}
