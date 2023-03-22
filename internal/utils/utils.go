package utils

import (
	"bufio"
	"io"

	"github.com/powerpuffpenguin/httpadapter/core"
	"github.com/powerpuffpenguin/httpadapter/internal/memory"
)

// 合併數據寫入
func Write(
	allocator memory.BufferAllocator,
	wc io.WriteCloser, size int,
	done <-chan struct{},
	active chan<- int,
	r <-chan memory.Buffer,
) {
	defer wc.Close()
	var (
		b  memory.Buffer
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
			_, e = w.Write(b.Data)
			allocator.Put(b)
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
				_, e = w.Write(b.Data)
				allocator.Put(b)
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

func OnPong(
	allocator memory.BufferAllocator,
	r io.Reader, buf []byte,
	done <-chan struct{},
	w chan<- []byte,
) (exit bool) {
	_, e := io.ReadFull(r, buf[1:5])
	if e != nil {
		exit = true
		return
	}
	data := make([]byte, 5)
	copy(data, buf[:5])
	select {
	case <-done:
		exit = true
		return
	case w <- data:
	}
	exit = core.PostWrite(done, w, data)
	return
}
