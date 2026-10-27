package templates

import (
	"bytes"
	"embed"
	"html/template"
	"io"
	"sync"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/clock"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/resources/assets"
	"github.com/soulteary/flare/internal/searchui"
)

//go:embed html
var TPL embed.FS

// Navigation pages can be several MiB. Retain reusable buffers for ordinary
// pages, but do not pin an oversized response in sync.Pool's victim generation.
const maxRetainedTemplateBuffer = 256 << 10

var bufPool = sync.Pool{
	New: func() any { return &bytes.Buffer{} },
}

// Renderer implements fiber.Views for HTML templates.
type Renderer struct {
	templates *template.Template
}

func (r *Renderer) Render(w io.Writer, templateName string, data any, _ ...string) error {
	tmplName := templateName
	for _, cand := range []string{templateName, "html/" + templateName, "embed/templates/" + templateName} {
		if r.templates.Lookup(cand) != nil {
			tmplName = cand
			break
		}
	}
	buf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok || buf == nil {
		buf = &bytes.Buffer{}
	}
	buf.Reset()
	defer releaseTemplateBuffer(buf)
	if err := r.templates.ExecuteTemplate(buf, tmplName, data); err != nil {
		return err
	}
	_, err := buf.WriteTo(w)
	return err
}

func releaseTemplateBuffer(buf *bytes.Buffer) {
	if buf.Cap() > maxRetainedTemplateBuffer {
		// Reset alone retains the backing array, including after WriteTo. Dropping
		// the allocation preserves the small pooled object without retaining HTML.
		*buf = bytes.Buffer{}
	} else {
		buf.Reset()
	}
	bufPool.Put(buf)
}

var templateFuncMap = template.FuncMap{
	"T":               i18n.T,
	"HTMLLang":        i18n.HTMLLang,
	"FaviconURL":      assets.FaviconURL,
	"ThemeFaviconURL": assets.ThemeFaviconURL,
}

// Load satisfies fiber.Views. NewRenderer parses templates before app creation.
func (r *Renderer) Load() error { return nil }

// NewRenderer prepares the view engine for fiber.Config.Views.
func NewRenderer() (fiber.Views, error) {
	var t *template.Template
	var err error
	if define.AppFlags.DebugMode {
		t, err = template.New("").Funcs(templateFuncMap).ParseGlob("embed/templates/*.html")
	} else {
		t, err = template.New("").Funcs(templateFuncMap).ParseFS(TPL, "html/*.html")
	}
	if err != nil {
		return nil, err
	}
	t, err = t.Parse(clock.Templates() + searchui.Templates())
	if err != nil {
		return nil, err
	}
	return &Renderer{templates: t}, nil
}
