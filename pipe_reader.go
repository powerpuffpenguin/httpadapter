package httpadapter

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

var errPipeReaderClosed = errors.New(`httpadapter: PipeReader closed`)

type pipeReader struct {
	rw *ReadWriter

	cond   *sync.Cond
	closed int32
	wait   int
}

func newPipeReader(size int) (pipe *pipeReader) {
	pipe = &pipeReader{
		rw:   NewReadWriter(make([]byte, size)),
		cond: sync.NewCond(&sync.Mutex{}),
	}
	return
}
func (p *pipeReader) Close() {
	if p.closed == 0 && atomic.SwapInt32(&p.closed, 1) == 0 {
		p.cond.L.Lock()
		defer p.cond.L.Unlock()

		if p.wait != 0 {
			// 存在 等待 goroutine 喚醒 她們
			p.cond.Broadcast()
		}
	}
}
func (p *pipeReader) Write(b []byte) (n int, e error) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	// 檢測關閉
	if p.closed != 0 {
		e = errPipeReaderClosed
		return
	}
	// 寫入數據
	n, e = p.rw.Write(b)
	if e != nil {
		return
	}

	if p.wait != 0 {
		// 存在 等待 goroutine 喚醒 她們
		p.cond.Broadcast()
	}
	return
}
func (p *pipeReader) Read(b []byte) (n int, e error) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	// 檢測關閉
	if len(b) == 0 {
		if p.closed != 0 {
			e = io.EOF
		}
		return
	}

	//  等待可讀數據
	rw := p.rw
	for rw.Len() == 0 {
		if p.closed != 0 {
			e = io.EOF
			return
		}
		p.wait++
		p.cond.Wait()
		p.wait--
	}

	// 讀取數據
	n, e = rw.Read(b)
	return
}
