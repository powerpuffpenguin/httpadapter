package pipe

import "io"

func ReadAll(r io.Reader, b []byte) (n int, e error) {
	n, e = io.ReadFull(r, b)
	if e == io.ErrUnexpectedEOF {
		e = nil
	}
	return
}
