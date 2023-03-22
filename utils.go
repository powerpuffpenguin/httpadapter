package httpadapter

import (
	"io"

	"github.com/powerpuffpenguin/httpadapter/core"
)

func onPong(r io.Reader, buf []byte,
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
		return
	default:
	}
	exit = core.PostWrite(done, w, data)
	return
}
