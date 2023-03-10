package httpadapter

type pipeReader struct {
	done   chan struct{}
	buffer []byte
}

func newPipeReader(done chan struct{}, size int) *pipeReader {
	return &pipeReader{
		done:   done,
		buffer: make([]byte, size),
	}
}
