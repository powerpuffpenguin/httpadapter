package httpadapter

import (
	"context"
	"net"
)

type Conn interface {
	net.Conn
	Context() context.Context
}
