package web

import (
	"bytes"
	_ "embed"

	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/powerpuffpenguin/httpadapter/document"
)

//go:embed markdown.css
var markdownCSS []byte
var (
	started                         = time.Now()
	httpadapter, forward, transport []byte
)

func init() {
	var reader = html.NewRenderer(html.RendererOptions{
		Flags: html.CompletePage | html.CommonFlags,
		CSS:   "/document/markdown.css",
	})
	reader.Opts.Title = `httpadapter 協議`
	httpadapter = markdown.ToHTML(document.Httpadapter,
		parser.NewWithExtensions(parser.CommonExtensions|parser.AutoHeadingIDs),
		reader,
	)
	reader.Opts.Title = `傳輸層`
	transport = markdown.ToHTML(document.Transport,
		parser.NewWithExtensions(parser.CommonExtensions|parser.AutoHeadingIDs),
		reader,
	)
	reader.Opts.Title = `中轉層`
	forward = markdown.ToHTML(document.Forward,
		parser.NewWithExtensions(parser.CommonExtensions|parser.AutoHeadingIDs),
		reader,
	)
}

type Document struct {
}

func (m Document) Register(router gin.IRouter) {
	r := router.Group(`document`)
	r.GET(``, m.index)
	r.GET(`markdown.css`, m.markdown)
	r.GET(`httpadapter.md`, m.httpadapter)
	r.GET(`transport.md`, m.transport)
	r.GET(`forward.md`, m.forward)
}
func (Document) index(c *gin.Context) {
	c.Redirect(http.StatusMovedPermanently, "/document/httpadapter.md")
}
func (Document) markdown(c *gin.Context) {
	c.Writer.Header().Set(`Content-Type`, `text/css; charset=utf-8`)
	http.ServeContent(c.Writer, c.Request,
		`markdown.css`, started,
		bytes.NewReader(markdownCSS),
	)
}
func (Document) httpadapter(c *gin.Context) {
	c.Writer.Header().Set(`Content-Type`, `text/html; charset=utf-8`)
	http.ServeContent(c.Writer, c.Request,
		`index.md`, started,
		bytes.NewReader(httpadapter),
	)
}
func (Document) transport(c *gin.Context) {
	c.Writer.Header().Set(`Content-Type`, `text/html; charset=utf-8`)
	http.ServeContent(c.Writer, c.Request,
		`transport.md`, started,
		bytes.NewReader(transport),
	)
}
func (Document) forward(c *gin.Context) {
	c.Writer.Header().Set(`Content-Type`, `text/html; charset=utf-8`)
	http.ServeContent(c.Writer, c.Request,
		`forward.md`, started,
		bytes.NewReader(forward),
	)
}
