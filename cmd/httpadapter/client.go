package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/powerpuffpenguin/httpadapter"
	"github.com/spf13/cobra"
)

func getHeader(vals []string) http.Header {
	if len(vals) == 0 {
		return nil
	}
	keys := make(http.Header, len(vals))
	for _, val := range vals {
		i := strings.Index(val, `=`)
		if i < 0 {
			log.Fatalln(`header not found =`, val)
		}
		keys.Add(val[:i], val[i+1:])
	}
	return keys
}

func client() *cobra.Command {
	var (
		uri, method, server string
		header              []string
		body                string
		usetls, insecure    bool
	)
	cmd := &cobra.Command{
		Use:   "client",
		Short: "httpadapter client, It can be used to send requests to the httpadapter server",
		Run: func(cmd *cobra.Command, args []string) {
			var opts []httpadapter.ClientOption
			if usetls {
				opts = append(opts, httpadapter.WithDialer(&tls.Dialer{
					Config: &tls.Config{
						InsecureSkipVerify: insecure,
					},
				}))
			}
			client := httpadapter.NewClient(server, opts...)
			u, e := url.Parse(uri)
			if e != nil {
				return
			}
			switch u.Scheme {
			case "http", "https":
				var bodylen = len(body)
				resp, e := client.Unary(context.Background(), &httpadapter.MessageRequest{
					URL:     uri,
					Method:  method,
					BodyLen: uint64(bodylen),
					Body:    strings.NewReader(body),
					Header:  getHeader(header),
				})
				if e != nil {
					log.Fatalln(e)
				}
				fmt.Println(`status:`, resp.Status)
				fmt.Println(`header:`, resp.Header)
				if resp.BodyLen > 0 {
					fmt.Print(`body: `)
					_, e := io.Copy(os.Stdout, resp.Body)
					resp.Body.Close()
					if e != nil {
						log.Fatalln(e)
					}
					fmt.Println()
				}
			case "ws", "wss":
				ws, _, e := client.Websocket(context.Background(), uri, getHeader(header))
				if e != nil {
					log.Fatalln(e)
				}
				defer ws.Close()
				var locker sync.Mutex
				go func() {
					r := bufio.NewReader(os.Stdin)
					for {
						text, b := getInput(&locker, r)
						if text {
							_, e = ws.WriteMessage(websocket.TextMessage, b)
						} else {
							_, e = ws.WriteMessage(websocket.BinaryMessage, b)
						}
						if e != nil {
							locker.Unlock()
							log.Fatalln(e)
							locker.Unlock()
						}
					}
				}()
				for {
					t, b, e := ws.ReadMessage()
					if e != nil {
						locker.Lock()
						log.Fatalln(e)
						locker.Unlock()
					}
					locker.Lock()
					log.Println(`recv:`, t, b)
					locker.Unlock()
				}
			case "tcp", "tls":
				c, _, e := client.Connect(context.Background(), uri)
				if e != nil {
					log.Fatalln(e)
				}
				defer c.Close()
				b := make([]byte, 1024)
				var locker sync.Mutex
				go func() {
					r := bufio.NewReader(os.Stdin)
					for {
						_, b = getInput(&locker, r)
						_, e = c.Write(b)
						if e != nil {
							locker.Lock()
							log.Fatalln(e)
							locker.Unlock()
						}
					}
				}()
				for {
					n, e := c.Read(b)
					if e != nil {
						locker.Lock()
						log.Fatalln(e)
						locker.Unlock()
					}
					locker.Lock()
					log.Println(`recv:`, b[:n])
					locker.Unlock()
				}
			}
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&body, `data`, `d`, ``, `send body`)
	flags.StringVarP(&uri, `url`, `u`, ``, `requested interface url`)
	flags.StringVarP(&method, `method`, `M`, http.MethodGet, `request method`)
	flags.StringSliceVarP(&header, `header`, `H`, nil, `request header key=val`)
	flags.StringVarP(&server, `server`, `s`, ``, `httpadapter server host:port`)
	flags.BoolVar(&usetls, `tls`, false, `connect use tls`)
	flags.BoolVarP(&insecure, `insecure`, `k`, false, `allow insecure server connections when using SSL`)
	return cmd
}
func getInput(locker sync.Locker, r *bufio.Reader) (text bool, b []byte) {

	var (
		first = true
		e     error
		s     string
	)
	for {
		locker.Lock()
		if first {
			fmt.Println(`send text: xxx`)
			fmt.Println(`send binary: hex:xxx or base64:xxx`)
			first = false
		}
		fmt.Println(`input send:`)
		locker.Unlock()
		s, e = r.ReadString('\n')
		if e != nil {
			locker.Lock()
			log.Fatalln(e)
			locker.Unlock()
		}
		s = strings.TrimSpace(s)
		if s == `` {
			continue
		}
		if strings.HasPrefix(s, `hex:`) {
			b, e = hex.DecodeString(s[len(`hex:`):])
			if e == nil {
				break
			}
			locker.Lock()
			log.Println(e)
			locker.Unlock()
		} else if strings.HasPrefix(s, `base64:`) {
			b, e = base64.StdEncoding.DecodeString(s[len(`base64:`):])
			if e == nil {
				break
			}
			locker.Lock()
			log.Println(e)
			locker.Unlock()
		} else {
			text = true
			b = []byte(s)
			break
		}
	}
	return
}
