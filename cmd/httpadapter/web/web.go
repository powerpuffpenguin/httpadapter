package web

import (
	"github.com/gin-gonic/gin"
)

type Module interface {
	Register(router gin.IRouter)
}
