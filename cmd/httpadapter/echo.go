package main

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/powerpuffpenguin/httpadapter"
	"github.com/spf13/cobra"
)

func echo() *cobra.Command {
	var (
		addr string
	)
	cmd := &cobra.Command{
		Use:   "echo",
		Short: "httpadapter echo server",
		Run: func(cmd *cobra.Command, args []string) {
			l, e := net.Listen(`tcp`, addr)
			if e != nil {
				log.Fatalln(e)
			}
			log.Println(`echo listen:`, addr)
			var opts = []httpadapter.ServerOption{
				httpadapter.ServerTCPDialer(httpadapter.TCPDialerFunc(func(ctx context.Context, addr string, tls bool) (c net.Conn, e error) {
					c0, c1 := net.Pipe()
					go func() {
						b := make([]byte, 1024)
						for {
							n, e := c1.Read(b)
							if e != nil {
								break
							}
							_, e = c1.Write(b[:n])
							if e != nil {
								break
							}
						}
						c1.Close()
					}()
					c = c0
					return
				})),
			}
			var upgrader = websocket.Upgrader{
				ReadBufferSize:  1024,
				WriteBufferSize: 1024,
				CheckOrigin: func(r *http.Request) bool {
					return true
				},
			}
			engine := gin.Default()
			engine.NoRoute(func(c *gin.Context) {
				if c.IsWebsocket() {
					ws, e := upgrader.Upgrade(c.Writer, c.Request, c.Writer.Header())
					if e != nil {
						c.String(http.StatusInternalServerError, e.Error())
						return
					}
					for {
						t, r, e := ws.NextReader()
						if e != nil {
							break
						}
						w, e := ws.NextWriter(t)
						if e != nil {
							break
						}
						_, e = io.Copy(w, r)
						if e != nil {
							break
						}
						e = w.Close()
						if e != nil {
							break
						}
					}
					ws.Close()
					return
				}
				b, e := io.ReadAll(c.Request.Body)
				if e != nil {
					c.String(http.StatusInternalServerError, e.Error())
					return
				}
				c.IndentedJSON(http.StatusOK, map[string]any{
					`url`:    c.Request.URL.String(),
					`header`: c.Request.Header,
					`body`:   string(b),
				})
			})
			opts = append(opts, httpadapter.ServerHTTP(registerWeb(engine)))
			httpadapter.NewServer(opts...).Serve(l)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&addr, `addr`, `a`, `:9000`, `echo server listen addr`)
	return cmd
}
