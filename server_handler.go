package httpadapter

import (
	"fmt"
	"net"
)

type Handler interface {
	ServeChannel(c net.Conn)
}
type handlerFunc struct {
	f func(c net.Conn)
}

func HandleFunc(f func(c net.Conn)) Handler {
	return handlerFunc{
		f: f,
	}
}
func (h handlerFunc) ServeChannel(c net.Conn) {
	h.f(c)
}

var defaultHandler = channelHandler{}

type channelHandler struct {
}

func (channelHandler) ServeChannel(c net.Conn) {
	fmt.Println(`ServeChannel`, c)
}
