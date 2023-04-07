package main

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/powerpuffpenguin/httpadapter"
	"github.com/spf13/cobra"
	"golang.org/x/net/http2"
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
			if cnf.Options.Backend == `` {
				opts = append(opts, httpadapter.ServerHTTP(registerWeb(gin.Default())))
			} else {
				opts = append(opts,
					httpadapter.ServerBackend(
						httpadapter.NewTCPBackend(cnf.Options.Backend),
					),
				)
			}
			if cnf.Options.Window > 0 {
				opts = append(opts,
					httpadapter.ServerWindow(
						cnf.Options.Window,
					),
				)
			}
			if cnf.Options.Timeout > 0 {
				opts = append(opts,
					httpadapter.ServerTimeout(
						cnf.Options.Timeout,
					),
				)
			}
			if cnf.Options.ReadBuffer > 0 {
				opts = append(opts,
					httpadapter.ServerReadBuffer(
						cnf.Options.ReadBuffer,
					),
				)
			}
			if cnf.Options.WriteBuffer > 0 {
				opts = append(opts,
					httpadapter.ServerReadBuffer(
						cnf.Options.WriteBuffer,
					),
				)
			}
			if cnf.Options.Channels > 0 {
				opts = append(opts,
					httpadapter.ServerChannels(
						cnf.Options.Channels,
					),
				)
			}
			if cnf.Options.Ping > 0 {
				opts = append(opts,
					httpadapter.ServerPing(
						cnf.Options.Ping,
					),
				)
			}
			// rewriter
			if len(cnf.Rewriter) != 0 {
				rewriter := make([]*UnaryRewriter, 0, len(cnf.Rewriter))
				for _, o := range cnf.Rewriter {
					r := newUnaryRewriter(&o)
					rewriter = append(rewriter, r)
					log.Println(r)
				}
				errAllowed := errors.New(`url not allowed`)
				opts = append(opts, httpadapter.ServerHookURL(httpadapter.HookURLFunc(func(u *url.URL) (nu *url.URL, e error) {
					for _, r := range rewriter {
						if r.Match(u) {
							if r.Reject {
								e = errAllowed
								return
							}
							if r.Scheme != `` {
								u.Scheme = r.Scheme
							}
							if r.Host != `` {
								u.Host = r.Host
							}
							nu = u
							return
						}
					}
					nu = u
					return
				})))
			}
			// h2c
			var (
				h2cHosts     []Matcher
				h2cHostnames []Matcher
				client       = &http.Client{
					CheckRedirect: func(req *http.Request, via []*http.Request) error {
						return http.ErrUseLastResponse
					},
				}
				h2cClient *http.Client
			)
			for _, s := range cnf.H2C.Hosts {
				m := newMatcher(s)
				h2cHosts = append(h2cHosts, m)
				log.Println(`h2c host`, m)
			}
			for _, s := range cnf.H2C.Hostnames {
				m := newMatcher(s)
				h2cHostnames = append(h2cHostnames, m)
				log.Println(`h2c hostname`, m)
			}
			if len(h2cHosts) != 0 || len(h2cHostnames) != 0 {
				h2cClient = &http.Client{
					Transport: &http2.Transport{
						DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
							return net.Dial(network, addr)
						},
						AllowHTTP: true,
					},
					CheckRedirect: func(req *http.Request, via []*http.Request) error {
						return http.ErrUseLastResponse
					},
				}
			}
			opts = append(opts, httpadapter.ServerHookDo(httpadapter.HookDoFunc(func(req *http.Request) (resp *http.Response, e error) {
				useh2c := false
				if h2cClient != nil {
					for _, m := range h2cHosts {
						if m.Match(req.URL.Host) {
							useh2c = true
							break
						}
					}
					if !useh2c {
						s := req.URL.Hostname()
						for _, m := range h2cHostnames {
							if m.Match(s) {
								useh2c = true
								break
							}
						}
					}
				}
				if useh2c {
					resp, e = h2cClient.Do(req)
				} else {
					resp, e = client.Do(req)
				}
				return
			})))
			httpadapter.NewServer(opts...).Serve(l)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&addr, `addr`, `a`, ``, `server listen addr`)
	flags.StringVarP(&cnfpath, `cnf`, `c`, filepath.Join(BasePath(), `etc`, `server.jsonnet`), `configure file`)
	return cmd
}

type Matcher interface {
	Match(s string) bool
}

func newMatcher(s string) Matcher {
	if s == `*` {
		return matcherAny{}
	} else if strings.HasPrefix(s, `^`) {
		return matcherPrefix(s[1:])
	} else if strings.HasSuffix(s, `$`) {
		return matcherSuffix(s[:len(s)-1])
	} else if strings.HasPrefix(s, `@`) {
		r, e := regexp.Compile(s[1:])
		if e != nil {
			log.Fatalln(e)
		}
		return &matcherRegexp{
			r: r,
			s: s[1:],
		}
	}
	return matcherEqual(s)
}

type matcherAny struct{}

func (matcherAny) Match(s string) bool {
	return true
}
func (matcherAny) String() string {
	return `any: *`
}

type matcherPrefix string

func (m matcherPrefix) Match(s string) bool {
	return strings.HasPrefix(s, string(m))
}
func (m matcherPrefix) String() string {
	return `prefix: ` + string(m)
}

type matcherSuffix string

func (m matcherSuffix) Match(s string) bool {
	return strings.HasSuffix(s, string(m))
}
func (m matcherSuffix) String() string {
	return `suffix: ` + string(m)
}

type matcherEqual string

func (m matcherEqual) Match(s string) bool {
	return s == string(m)
}
func (m matcherEqual) String() string {
	return `equal: ` + string(m)
}

type matcherRegexp struct {
	r *regexp.Regexp
	s string
}

func (m *matcherRegexp) Match(s string) bool {
	return m.r.MatchString(s)
}
func (m *matcherRegexp) String() string {
	return `regexp: ` + m.s
}

type UnaryRewriter struct {
	Reject       bool
	Scheme, Host string

	Schemes, Hosts, Hostnames []Matcher
}

func newUnaryRewriter(o *Rewriter) *UnaryRewriter {
	var (
		schemes   = make([]Matcher, 0, len(o.Schemes))
		hosts     = make([]Matcher, 0, len(o.Hosts))
		hostnames = make([]Matcher, 0, len(o.Hostnames))
	)
	for _, s := range o.Schemes {
		schemes = append(schemes, newMatcher(s))
	}
	for _, s := range o.Hosts {
		hosts = append(hosts, newMatcher(s))
	}
	for _, s := range o.Hostnames {
		hostnames = append(hostnames, newMatcher(s))
	}

	return &UnaryRewriter{
		Reject:    o.Reject,
		Scheme:    o.Scheme,
		Host:      o.Host,
		Schemes:   schemes,
		Hosts:     hosts,
		Hostnames: hostnames,
	}
}
func (r *UnaryRewriter) match(ms []Matcher, s string) bool {
	for _, m := range ms {
		if m.Match(s) {
			return true
		}
	}
	return false
}
func (r *UnaryRewriter) Match(u *url.URL) bool {
	if len(r.Schemes) != 0 && !r.match(r.Schemes, u.Scheme) {
		return false
	}
	if len(r.Hosts) != 0 && !r.match(r.Hosts, u.Host) {
		return false
	}
	if len(r.Hostnames) != 0 {
		s := u.Hostname()
		if !r.match(r.Hostnames, s) {
			return false
		}
	}
	return true
}
func (r *UnaryRewriter) String() string {
	b := bytes.NewBufferString(`rewriter`)
	if r.Reject {
		b.WriteString(` reject`)
	} else if r.Scheme != `` {
		if r.Host == `` {
			b.WriteString(` ` + r.Scheme)
		} else {
			b.WriteString(` ` + r.Scheme + `://` + r.Host)
		}
	} else if r.Host != `` {
		b.WriteString(` ` + r.Host)
	}
	if len(r.Schemes) != 0 {
		b.WriteString(fmt.Sprintf(` schemes=%v`, r.Schemes))
	}
	if len(r.Hosts) != 0 {
		b.WriteString(fmt.Sprintf(` hosts=%v`, r.Hosts))
	}
	if len(r.Hostnames) != 0 {
		b.WriteString(fmt.Sprintf(` hostnames=%v`, r.Hostnames))
	}
	return b.String()
}
