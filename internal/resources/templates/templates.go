package templates

import (
	"bytes"
	"embed"
	"html/template"
	"io"
	"sync"

	"github.com/labstack/echo/v5"

	FlareDefine "github.com/soulteary/flare/config/define"
)

//go:embed html
var TPL embed.FS

var bufPool = sync.Pool{
	New: func() any { return &bytes.Buffer{} },
}

// Renderer implements echo.Renderer for HTML templates.
type Renderer struct {
	templates *template.Template
}

func (r *Renderer) Render(c *echo.Context, w io.Writer, templateName string, data any) error {
	tmplName := templateName
	for _, cand := range []string{templateName, "html/" + templateName, "embed/templates/" + templateName} {
		if r.templates.Lookup(cand) != nil {
			tmplName = cand
			break
		}
	}
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)
	if err := r.templates.ExecuteTemplate(buf, tmplName, data); err != nil {
		return err
	}
	_, err := buf.WriteTo(w)
	return err
}

func RegisterRouting(e *echo.Echo) {
	var t *template.Template
	if FlareDefine.AppFlags.DebugMode {
		t = template.Must(template.ParseGlob("embed/templates/*.html"))
	} else {
		t = template.Must(template.New("").ParseFS(TPL, "html/*.html"))
	}
	e.Renderer = &Renderer{templates: t}
}
