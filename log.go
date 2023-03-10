package httpadapter

import (
	"log"
	"os"
)

var ServerLogger = log.New(os.Stdout, `[httpadapter-server] `, log.Lshortfile|log.LstdFlags)
var ClientLogger = log.New(os.Stdout, `[httpadapter-client] `, log.Lshortfile|log.LstdFlags)
