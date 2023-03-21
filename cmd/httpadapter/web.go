package main

import (
	"httpadapter/web"
	"net/http"
	"net/http/pprof"

	"github.com/gin-gonic/gin"
)

func registerWeb(engine *gin.Engine) http.Handler {
	var modules = []web.Module{
		web.Document{},
	}
	for _, m := range modules {
		m.Register(engine)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(`/`, func(w http.ResponseWriter, r *http.Request) {
		engine.ServeHTTP(w, r)
	})
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}
