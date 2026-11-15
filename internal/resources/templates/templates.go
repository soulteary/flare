package templates

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"html/template"
	"io"
	"sync"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/clock"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/resources/assets"
	"github.com/soulteary/flare/internal/searchui"
	"github.com/soulteary/flare/internal/themetemplate"
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
	// home renders home.html the way a page template of a theme is
	// rendered. It is nil for a set of templates without that page.
	home *homePage
}

// homePage is home.html taken apart: the skeleton that places the parts,
// compiled once, and the page around it.
type homePage struct {
	// skeleton is the text between the parts, with a hole for each.
	skeleton *themetemplate.Fragment
	// page is the set of templates with the skeleton replaced by mark.
	page *template.Template
	// mark stands in the rendered page where the skeleton belongs. It is
	// made anew for each start, and nothing a request brings can hold it:
	// what the page shows outside its skeleton is escaped.
	mark template.HTML
}

const (
	homeTemplate = "home.html"
	// homeSkeleton is the block of home.html that places the parts.
	homeSkeleton = "home/skeleton"
	// homeBody is the key under which the page gets the mark.
	homeBody = "PageBody"
	// homeTitle is the key of the title of the site, the one field a page
	// template of a theme shows.
	homeTitle = "OptionTitle"
)

// prepareHome compiles the skeleton of home.html. It runs before any template
// of the set is executed, because a set cannot be cloned afterwards.
func prepareHome(templates *template.Template) (*homePage, error) {
	marked, err := templates.Clone()
	if err != nil {
		return nil, err
	}
	skeleton, err := themetemplate.Builtin(marked, homeSkeleton, HomeComponents, nil)
	if err != nil {
		return nil, err
	}
	page, err := templates.Clone()
	if err != nil {
		return nil, err
	}
	if _, err = page.New(homeSkeleton).Parse("{{." + homeBody + "}}"); err != nil {
		return nil, err
	}
	var raw [16]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return nil, err
	}
	// #nosec G203 -- a comment with a random name, written by the application.
	return &homePage{skeleton: skeleton, page: page, mark: template.HTML("<!--flare-body-" + hex.EncodeToString(raw[:]) + "-->")}, nil
}

// parts renders the parts of one page where the skeleton has them.
type parts struct {
	templates *template.Template
	data      map[string]any
	// err is the first failure of a part. Nothing is rendered after it.
	err error
}

// Text is the title of the site, which is all a page template can show.
func (p *parts) Text(string) string {
	if title, set := p.data[homeTitle].(string); set {
		return title
	}
	return ""
}

func (p *parts) Address() string                   { return "" }
func (p *parts) LinkAttributes() template.HTMLAttr { return "" }
func (p *parts) Slot(string) template.HTML         { return "" }

// WriteSlot renders a part straight into the page, so that a long list of
// entries is not copied on its way there.
func (p *parts) WriteSlot(out io.Writer, name string) {
	if p.err == nil {
		p.err = p.templates.ExecuteTemplate(out, name, p.data)
	}
}

var errHomeMark = errors.New("the page does not hold the place of its skeleton")

// render writes the page: the page around the skeleton is rendered with a
// mark in its place, and the skeleton is written where the mark stands, each
// part rendered by itself into its hole. Where a theme brings the template
// of the page, its skeleton stands in place of the one of home.html; the
// parts are rendered the same way, and the page around them is the same.
func (h *homePage) render(buf *bytes.Buffer, templates *template.Template, data map[string]any) error {
	skeleton := h.skeleton
	if custom, given := data[themetemplate.PageData].(*themetemplate.Fragment); given && custom != nil {
		skeleton = custom
	}
	around := getTemplateBuffer()
	defer releaseTemplateBuffer(around)
	data[homeBody] = h.mark
	err := h.page.ExecuteTemplate(around, homeTemplate, data)
	delete(data, homeBody)
	if err != nil {
		return err
	}
	at := bytes.Index(around.Bytes(), []byte(h.mark))
	if at < 0 {
		return errHomeMark
	}
	buf.Write(around.Bytes()[:at])
	filled := parts{templates: templates, data: data}
	skeleton.Render(buf, &filled)
	if filled.err != nil {
		return filled.err
	}
	buf.Write(around.Bytes()[at+len(h.mark):])
	return nil
}

func getTemplateBuffer() *bytes.Buffer {
	buf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok || buf == nil {
		buf = &bytes.Buffer{}
	}
	buf.Reset()
	return buf
}

func (r *Renderer) Render(w io.Writer, templateName string, data any, _ ...string) error {
	tmplName := templateName
	for _, cand := range []string{templateName, "html/" + templateName, "embed/templates/" + templateName} {
		if r.templates.Lookup(cand) != nil {
			tmplName = cand
			break
		}
	}
	buf := getTemplateBuffer()
	defer releaseTemplateBuffer(buf)
	if values, ok := data.(map[string]any); ok && r.home != nil && tmplName == homeTemplate {
		if err := r.home.render(buf, r.templates, values); err != nil {
			return err
		}
	} else if err := r.templates.ExecuteTemplate(buf, tmplName, data); err != nil {
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

// HomeComponents names the parts that the home page, the application page and
// the bookmark page are put together from, in the order home.html has them.
// Each is a template of that name, defined in place in home.html, so that a
// part can be rendered by itself. A part includes the condition under which
// the site shows it: it renders nothing when its module is turned off.
var HomeComponents = []string{
	"flare/search",
	"flare/clock",
	"flare/greeting",
	"flare/weather",
	"flare/subpage-header",
	"flare/apps",
	"flare/bookmarks",
	"flare/toolbar",
	"flare/footer",
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
	home, err := prepareHome(t)
	if err != nil {
		return nil, err
	}
	return &Renderer{templates: t, home: home}, nil
}
