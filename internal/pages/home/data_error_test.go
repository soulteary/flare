package home

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/data"
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

func TestNavigationErrorExplainsWithheldPrivateContent(t *testing.T) {
	err := fmt.Errorf("读取配置文件 bookmarks: %w", data.ErrPrivateContentRequiresLogin)
	for _, module := range []string{"applications", "bookmarks"} {
		if message := navigationDataError(module, "en", err); message != "Private entries exist while login is disabled. Enable login, or remove the private flags from the YAML files." {
			t.Fatalf("unexpected %s message: %q", module, message)
		}
	}
	if message := navigationDataError("bookmarks", "zh", err); message != i18n.T("zh", "navigation.private_requires_login") || message == "navigation.private_requires_login" {
		t.Fatalf("missing Chinese message: %q", message)
	}
}
