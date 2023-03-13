package httpadapter

import (
	"fmt"
	"net"
)

type Handler interface {
	ServeChannel(c net.Conn)
}

var defaultHandler = channelHandler{}

type channelHandler struct {
}

func (channelHandler) ServeChannel(c net.Conn) {
	fmt.Println(`ServeChannel`, c)
}
