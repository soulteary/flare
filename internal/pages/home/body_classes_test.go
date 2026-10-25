package home

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/model"
)

// TestHomepageBodyClassesRemainPlainText guards the template auto-escaping boundary.
func TestHomepageBodyClassesRemainPlainText(t *testing.T) {
	injected := `compact\" onclick=\"alert(1)`
	options := model.Application{KeepLetterCase: true, Homepage: &model.HomepageOptions{
		DesktopColumns: injected, MobileBookmarkLayout: injected,
		NavigationTextSize: injected, TitleSize: injected,
	}}
	classes := homepageBodyClasses(options)
	if classes != "" {
		t.Fatalf("invalid options produced classes: %q", classes)
	}
	// Trusted template attribute types must not bypass automatic escaping.
	if _, ok := any(classes).(string); !ok {
		t.Fatal("body classes must remain a plain string")
	}
	text := classes
	page := template.Must(template.New("body").Parse(`<body class="{{.}}"></body>`))
	var output bytes.Buffer
	if err := page.Execute(&output, text+`" onclick="alert(1)`); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), `class="" onclick=`) || !strings.Contains(output.String(), "&#34;") {
		t.Fatalf("attribute value was not escaped: %s", output.String())
	}
	options = model.Application{Homepage: &model.HomepageOptions{MobileBookmarkLayout: "compact", DesktopColumns: "6"}}
	if got := homepageBodyClasses(options); got != "desktop-columns-6 mobile-bookmarks-compact app-content-uppercase " {
		t.Fatalf("unexpected classes: %q", got)
	}
}
