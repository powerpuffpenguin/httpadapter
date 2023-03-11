package httpadapter

import (
	"io"
	"time"
)

func Copy(dst, src io.ReadWriteCloser, b []byte) {
	if len(b) == 0 {
		b = make([]byte, 1024*32)
	}
	for {
		n, e := src.Read(b)
		if e != nil {
			src.Close()
			time.Sleep(time.Second)
			dst.Close() // 等待 dst 寫入完成
			break
		}
		_, e = dst.Write(b[:n])
		if e != nil {
			dst.Close()
			time.Sleep(time.Second)
			src.Close() // 等待 src 寫入完成
			break
		}
	}
}
