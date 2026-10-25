// Package searchui embeds the optional progressive search enhancement.
package searchui

import (
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"html/template"
	"strings"
)

//go:embed scripts/search.js
var source string

// html/template removes JavaScript comments. Hash the bytes it actually renders.
var script = func() string {
	t := template.Must(template.New("search-source").Parse("<script>" + source + "</script>"))
	var out strings.Builder
	if err := t.Execute(&out, nil); err != nil {
		panic(err)
	}
	return strings.TrimSuffix(strings.TrimPrefix(out.String(), "<script>"), "</script>")
}()

func Templates() string { return `{{define "search-hybrid-script"}}` + script + `{{end}}` }
func Hash() string {
	sum := sha256.Sum256([]byte(script))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}
