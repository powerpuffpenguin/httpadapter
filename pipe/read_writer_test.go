package pipe_test

import (
	"io"
	"testing"

	"github.com/powerpuffpenguin/httpadapter/pipe"
	"github.com/stretchr/testify/assert"
)

type helperReadWriter struct {
	t  *testing.T
	rw *pipe.ReadWriter
}

func (h *helperReadWriter) MustWriteString(s string) int {
	t := h.t
	rw := h.rw

	n, e := rw.WriteString(s)
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	if !assert.Equal(t, len(s), n) {
		t.FailNow()
	}
	return n
}
func (h *helperReadWriter) WriteStringError(s string, e error) {
	t := h.t
	rw := h.rw

	n, e1 := rw.WriteString(s)
	if !assert.Equal(t, e, e1) {
		t.FailNow()
	}
	if !assert.Equal(t, 0, n) {
		t.FailNow()
	}
}
func TestReadWriter(t *testing.T) {
	buffer := make([]byte, 5)
	copy(buffer, []byte("aaaaa"))
	rw := pipe.NewReadWriter(buffer)
	h := &helperReadWriter{
		t:  t,
		rw: rw,
	}
	size := 0
	size += h.MustWriteString("12")
	if !assert.Equal(t, size, rw.Len()) {
		t.FailNow()
	}
	if !assert.Equal(t, size, rw.Len()) {
		t.FailNow()
	}
	if !assert.Equal(t, "12aaa", string(buffer)) {
		t.FailNow()
	}

	size += h.MustWriteString("34")
	if !assert.Equal(t, size, rw.Len()) {
		t.FailNow()
	}
	if !assert.Equal(t, size, rw.Len()) {
		t.FailNow()
	}
	if !assert.Equal(t, "1234a", string(buffer)) {
		t.FailNow()
	}

	h.WriteStringError("56", pipe.ErrWriteOverflow)
	if !assert.Equal(t, size, rw.Len()) {
		t.FailNow()
	}

	size += h.MustWriteString("5")
	if !assert.Equal(t, size, rw.Len()) {
		t.FailNow()
	}
	if !assert.Equal(t, "12345", string(buffer)) {
		t.FailNow()
	}

	b := make([]byte, 2)
	n, e := io.ReadFull(rw, b)
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	if !assert.Equal(t, 2, n) {
		t.FailNow()
	}
	if !assert.Equal(t, 3, rw.Len()) {
		t.FailNow()
	}

	h.MustWriteString("09")
	if !assert.Equal(t, size, rw.Len()) {
		t.FailNow()
	}
	if !assert.Equal(t, "09345", string(buffer)) {
		t.FailNow()
	}
}
func TestReadWriterLoop(t *testing.T) {
	buffer := make([]byte, 5)
	copy(buffer, []byte("aaaaa"))
	rw := pipe.NewReadWriter(buffer)
	h := &helperReadWriter{
		t:  t,
		rw: rw,
	}
	size := 0
	size += h.MustWriteString("12")
	if !assert.Equal(t, size, rw.Len()) {
		t.FailNow()
	}
	if !assert.Equal(t, size, rw.Len()) {
		t.FailNow()
	}
	if !assert.Equal(t, "12aaa", string(buffer)) {
		t.FailNow()
	}

	size += h.MustWriteString("34")
	if !assert.Equal(t, size, rw.Len()) {
		t.FailNow()
	}
	if !assert.Equal(t, size, rw.Len()) {
		t.FailNow()
	}
	if !assert.Equal(t, "1234a", string(buffer)) {
		t.FailNow()
	}

	b := make([]byte, 2)
	n, e := io.ReadFull(rw, b)
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	if !assert.Equal(t, 2, n) {
		t.FailNow()
	}
	if !assert.Equal(t, 2, rw.Len()) {
		t.FailNow()
	}

	h.MustWriteString("098")
	if !assert.Equal(t, 5, rw.Len()) {
		t.FailNow()
	}
	if !assert.Equal(t, "98340", string(buffer)) {
		t.FailNow()
	}
}
