package httpadapter

type pipeReader struct {
	done chan struct{}
	rw   *ReadWriter
}

func newPipeReader(done chan struct{}, size int) (pipe *pipeReader) {
	pipe = &pipeReader{
		rw: NewReadWriter(make([]byte, size)),
	}
	return
}
func (p *pipeReader) Serve() {

}
func (p *pipeReader) Write(b []byte) (n int, e error) {

	return
}
func (p *pipeReader) Read(b []byte) (n int, e error) {

	return
}
