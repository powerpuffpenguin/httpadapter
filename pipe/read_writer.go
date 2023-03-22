package pipe

import (
	"errors"
	"io"

	"github.com/powerpuffpenguin/easygo/bytes"
)

var ErrWriteOverflow = errors.New(`httpadapter: ReadWriter write overflow`)
var ErrReadWriterClosed = errors.New("httpadapter: ReadWriter closed")

// 在內存中實現讀寫 pipe，它通常作爲底層的 pipe 實現
type ReadWriter struct {
	buffer []byte
	offset int
	size   int
}

func NewReadWriter(buffer []byte) *ReadWriter {
	return &ReadWriter{
		buffer: buffer,
	}
}

// 將數據寫入緩衝區，如果緩衝區沒有足夠可用空間將返回 0,ErrWriteOverflow 並且什麼都不寫入
func (rw *ReadWriter) WriteString(s string) (int, error) {
	return rw.Write(bytes.StringToBytes(s))
}

// 清空緩衝區中的數據
func (rw *ReadWriter) Clear() {
	rw.offset = 0
	rw.size = 0
}

// 返回緩衝區中數據大小
func (rw *ReadWriter) Len() int {
	return rw.size
}

// 返回可用緩衝區大小
func (rw *ReadWriter) Available() int {
	return len(rw.buffer) - rw.size
}

// 將數據寫入緩衝區，如果緩衝區沒有足夠可用空間將返回 0,ErrWriteOverflow 並且什麼都不寫入
func (rw *ReadWriter) Write(b []byte) (n int, e error) {
	size := len(b)
	if size == 0 {
		return
	}
	capacity := len(rw.buffer)
	available := capacity - rw.size
	if available < size {
		e = ErrWriteOverflow
		return
	}
	n = copy(rw.buffer[(rw.offset+rw.size)%capacity:], b)
	n += copy(rw.buffer, b[n:])

	rw.size += n
	return
}

// 讀取緩衝區中的數據，如果緩衝區中沒有數據立刻返回 0,EOF
func (rw *ReadWriter) Read(b []byte) (n int, e error) {
	if rw.size == 0 {
		e = io.EOF
		return
	}
	var readed int
	for rw.size != 0 && len(b) != 0 {
		readed = rw.read(b)
		b = b[readed:]
		n += readed
	}
	return
}
func (rw *ReadWriter) read(b []byte) (n int) {
	end := rw.offset + rw.size
	capacity := len(rw.buffer)
	if end > capacity {
		end = capacity
	}
	n = copy(b, rw.buffer[rw.offset:end])
	rw.size -= n
	rw.offset = (rw.offset + n) % capacity
	return
}
