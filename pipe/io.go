package pipe

import (
	"io"
	"time"
)

func Copy(
	dst io.WriteCloser, src io.ReadCloser,
	b []byte,
) (e error) {
	if len(b) == 0 {
		b = make([]byte, 1024*32)
	}
	var n int
	for {
		n, e = src.Read(b)
		if e != nil {
			src.Close()
			time.Sleep(time.Second * 3)
			dst.Close() // 等待 dst 寫入完成
			break
		}
		_, e = dst.Write(b[:n])
		if e != nil {
			dst.Close()
			time.Sleep(time.Second * 3)
			src.Close() // 等待 src 寫入完成
			break
		}
	}
	return
}
