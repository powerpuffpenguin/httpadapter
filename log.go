package httpadapter

import (
	"log"
	"os"
)

var Logger = log.New(os.Stdout, `[httpadapter] `, log.Lshortfile|log.LstdFlags)
