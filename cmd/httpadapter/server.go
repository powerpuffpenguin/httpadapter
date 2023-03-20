package main

import (
	"crypto/tls"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/powerpuffpenguin/httpadapter"
	"github.com/spf13/cobra"
)

func BasePath() string {
	filename, e := exec.LookPath(os.Args[0])
	if e != nil {
		log.Fatalln(e)
	}

	filename, e = filepath.Abs(filename)
	if e != nil {
		log.Fatalln(e)
	}
	return filepath.Dir(filename)
}

func server() *cobra.Command {
	var (
		addr    string
		cnfpath string
	)
	cmd := &cobra.Command{
		Use:   "server",
		Short: "run http adapter server",
		Run: func(cmd *cobra.Command, args []string) {
			var cnf *Server
			var e error
			if cnfpath == `` {
				cnf = &Server{
					Listen: `:8000`,
				}
			} else {
				cnf, e = LoadServer(cnfpath)
				if e != nil {
					log.Fatalln(e)
				}
			}
			if addr != `` {
				cnf.Listen = addr
			}
			var l net.Listener
			if cnf.CertFile != `` && cnf.KeyFile != `` {
				cert, e := tls.LoadX509KeyPair(cnf.CertFile, cnf.KeyFile)
				if e != nil {
					log.Fatalln(e)
				}
				l, e = tls.Listen(`tcp`, cnf.Listen, &tls.Config{
					//配置 證書
					Certificates: []tls.Certificate{cert},
				})
				if e != nil {
					log.Fatalln(e)
				}

				log.Println(`server tls listen:`, cnf.Listen)
			} else {
				l, e = net.Listen(`tcp`, cnf.Listen)
				if e != nil {
					log.Fatalln(e)
				}
				log.Println(`server tcp listen:`, cnf.Listen)
			}
			var opts []httpadapter.ServerOption
			if cnf.Options.Backend != `` {
				httpadapter.ServerBackend(
					httpadapter.NewTCPBackend(cnf.Options.Backend),
				)
			}
			if cnf.Options.Window > 0 {
				httpadapter.ServerWindow(
					cnf.Options.Window,
				)
			}
			if cnf.Options.Timeout > 0 {
				httpadapter.ServerTimeout(
					cnf.Options.Timeout,
				)
			}
			if cnf.Options.ReadBuffer > 0 {
				httpadapter.ServerReadBuffer(
					cnf.Options.ReadBuffer,
				)
			}
			if cnf.Options.WriteBuffer > 0 {
				httpadapter.ServerReadBuffer(
					cnf.Options.WriteBuffer,
				)
			}
			if cnf.Options.Channels > 0 {
				httpadapter.ServerChannels(
					cnf.Options.Channels,
				)
			}
			if cnf.Options.Ping > 0 {
				httpadapter.ServerPing(
					cnf.Options.Ping,
				)
			}
			httpadapter.NewServer(opts...).Serve(l)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&addr, `addr`, `a`, ``, `server listen addr`)
	flags.StringVarP(&cnfpath, `cnf`, `c`, filepath.Join(BasePath(), `server.jsonnet`), `configure file`)
	return cmd
}
