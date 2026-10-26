package home

import (
	"bytes"
	"errors"
	"html/template"
	"strings"
	"testing"

	"github.com/soulteary/flare/internal/i18n"
)

func TestNavigationErrorsRenderAsEscapedText(t *testing.T) {
	tmpl, err := template.New("home.html").Funcs(template.FuncMap{
		"T": i18n.T, "HTMLLang": i18n.HTMLLang, "ThemeFaviconURL": func() string { return "/favicon.ico" },
	}).ParseFiles("../../../embed/templates/home.html")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpl.Parse(`{{define "clock-script"}}{{end}}{{define "search-hybrid-script"}}{{end}}`); err != nil {
		t.Fatal(err)
	}
	message := `Unable to load <script>alert("x")</script> & data`
	var output bytes.Buffer
	if err := tmpl.Execute(&output, map[string]any{"Locale": "en", "OptionShowApps": true, "OptionShowBookmarks": true, "ApplicationsError": message, "BookmarksError": message}); err != nil {
		t.Fatal(err)
	}
	body := output.String()
	if strings.Count(body, `class="navigation-data-error" role="alert"`) != 2 {
		t.Fatal("both failed collections should show alerts")
	}
	if strings.Contains(body, `<script>alert`) || !strings.Contains(body, `&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt; &amp; data`) {
		t.Fatal("navigation error text was not escaped")
	}
}

func TestNavigationErrorHidesDiagnosticDetails(t *testing.T) {
	message := navigationDataError("bookmarks", "en", errors.New("invalid YAML in /private/config.yml <script>"))
	if message != "Unable to load saved bookmarks. Check the configuration and server logs." {
		t.Fatalf("unexpected safe message: %q", message)
	}
}
