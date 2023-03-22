package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/powerpuffpenguin/httpadapter"
	"github.com/powerpuffpenguin/httpadapter/pipe"
	"github.com/spf13/cobra"
)

func tunnel() *cobra.Command {
	var (
		debug   string
		cnfpath string
	)
	cmd := &cobra.Command{
		Use:   "tunnel",
		Short: "create a tunnel to httpadapter server backend",
		Run: func(cmd *cobra.Command, args []string) {
			cnfs, e := LoadTunnel(cnfpath)
			if e != nil {
				log.Fatalln(e)
			} else if len(cnfs) == 0 {
				log.Fatalln(`cnf nil`)
			}
			srvs := make([]*TunnelServer, 0, len(cnfs))
			keys := make(map[string]*httpadapter.Client, len(cnfs))
			for _, cnf := range cnfs {
				srv, e := NewTunnelServer(&cnf, keys)
				if e != nil {
					log.Fatalln(e)
				}
				srvs = append(srvs, srv)
			}
			var wait sync.WaitGroup
			if debug != `` {
				l, e := net.Listen(`tcp`, debug)
				if e != nil {
					log.Fatalln(e)
				}
				defer l.Close()

				engine := gin.Default()
				mux := registerWeb(engine)
				wait.Add(1)
				go func() {
					http.Serve(l, mux)
					wait.Done()
				}()
			}

			var pool sync.Pool
			pool.New = func() any {
				return make([]byte, 1024*32)
			}
			wait.Add(len(srvs))
			for _, srv := range srvs {
				go func(srv *TunnelServer) {
					defer wait.Done()
					srv.Serve(&pool)
				}(srv)
			}
			wait.Wait()
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&cnfpath, `cnf`, `c`, filepath.Join(BasePath(), `etc`, `tunnel.jsonnet`), `configure file`)
	flags.StringVarP(&debug, `debug`, `d`, ``, `if not empty, run debug http server ad this addr. like ':8080'`)
	return cmd
}

type TunnelServer struct {
	client  *httpadapter.Client
	l       net.Listener
	backend string
}

func NewTunnelServer(cnf *Tunnel, keys map[string]*httpadapter.Client) (srv *TunnelServer, e error) {
	if cnf.Server == `` {
		log.Fatalln(`invalid backend`, cnf.Server)
	}
	if cnf.Backend == `` {
		log.Fatalln(`invalid backend`, cnf.Backend)
	}
	var opts []httpadapter.ClientOption
	if cnf.Window > 0 {
		opts = append(opts, httpadapter.WithWindow(cnf.Window))
	}
	if cnf.ReadBuffer > 0 {
		opts = append(opts, httpadapter.WithReadBuffer(cnf.ReadBuffer))
	}
	if cnf.WriteBuffer > 0 {
		opts = append(opts, httpadapter.WithWriteBuffer(cnf.WriteBuffer))
	}
	if cnf.Ping > 0 {
		opts = append(opts, httpadapter.WithPing(cnf.Ping))
	}
	b, e := json.Marshal(cnf.Server)
	if e != nil {
		return
	}
	key := string(b)
	client, ok := keys[key]
	if !ok {
		client = httpadapter.NewClient(cnf.Server, opts...)
		keys[key] = client
	}
	l, e := net.Listen(`tcp`, cnf.Listen)
	if e != nil {
		return
	}
	log.Println(`tunnel listen:`, cnf.Listen)
	log.Println(`tunnel connect server:`, cnf.Server)
	log.Println(`tunnel server connect:`, cnf.Backend)
	srv = &TunnelServer{
		client:  client,
		l:       l,
		backend: cnf.Backend,
	}
	return
}
func (t *TunnelServer) Serve(pool *sync.Pool) {
	var (
		client  = t.client
		l       = t.l
		backend = t.backend
	)
	for {
		c, e := l.Accept()
		if e != nil {
			continue
		}
		go func(c net.Conn) {
			c0, _, e := client.Connect(context.Background(), backend)
			if e != nil {
				c.Close()
				if e != nil {
					log.Println(e)
				}
				return
			}
			b0 := pool.Get()
			b1 := pool.Get()

			go func() {
				pipe.Copy(c0, c, b0.([]byte))
				pool.Put(b0)
			}()
			pipe.Copy(c, c0, b1.([]byte))
			pool.Put(b1)
		}(c)
	}
}
