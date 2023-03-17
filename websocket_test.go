package httpadapter_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/powerpuffpenguin/httpadapter"
	"github.com/stretchr/testify/assert"
)

func TestClientWebsocket(t *testing.T) {
	var upgrader = websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc(`/ws`, func(w http.ResponseWriter, r *http.Request) {
		ws, e := upgrader.Upgrade(w, r, nil)
		if e != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set(`Content-Type`, `text/plain; charset=utf-8`)
			w.Write([]byte(e.Error()))
			return
		}
		defer ws.Close()
		for {
			t, p, e := ws.ReadMessage()
			if e != nil {
				break
			}
			e = ws.WriteMessage(t, p)
			if e != nil {
				break
			}
		}
	})

	s := newServer(t,
		httpadapter.ServerWindow(4),
		httpadapter.ServerHTTP(mux),
	)
	defer s.CloseAndWait()

	client := httpadapter.NewClient(Addr)
	defer client.Close()
	ws, resp, e := client.Websocket(context.Background(),
		BaseWebsocket+`/ws`,
		nil,
	)
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	if !assert.Equal(t, resp.Status, http.StatusSwitchingProtocols) {
		t.FailNow()
	}

	// text
	n, e := ws.WriteText("ok12")
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	if !assert.Equal(t, 4, n) {
		t.FailNow()
	}

}
