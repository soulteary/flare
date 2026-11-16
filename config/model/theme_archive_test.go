package model

import (
	"strings"
	"testing"
)

// An entry of a theme archive is a file of a theme by its path alone: one of
// two fixed names, a file with the extension of an image or a font in the
// assets, or a text document beside the theme file. Anything else is no file
// of a theme, and a path is taken as it is written.
func TestThemeArchiveEntry(t *testing.T) {
	for path, want := range map[string]struct {
		kind ThemeArchiveKind
		name string
	}{
		"theme.yml": {ThemeArchiveManifest, ""},
		"theme.css": {ThemeArchiveStylesheet, ""},
		// Assets go by their extension, in any case. Whether the name and
		// the content are accepted is for the reader of the theme.
		"assets/paper.png":    {ThemeArchiveAsset, "paper.png"},
		"assets/Brand.WOFF2":  {ThemeArchiveAsset, "Brand.WOFF2"},
		"assets/my photo.jpg": {ThemeArchiveAsset, "my photo.jpg"},
		"assets/a.b.c.avif":   {ThemeArchiveAsset, "a.b.c.avif"},
		// Text documents beside the theme file.
		"LICENSE":       {ThemeArchiveDocument, "LICENSE"},
		"NOTICE":        {ThemeArchiveDocument, "NOTICE"},
		"COPYING":       {ThemeArchiveDocument, "COPYING"},
		"README":        {ThemeArchiveDocument, "README"},
		"OFL.txt":       {ThemeArchiveDocument, "OFL.txt"},
		"notes.md":      {ThemeArchiveDocument, "notes.md"},
		"README.MD":     {ThemeArchiveDocument, "README.MD"},
		"LICENSE-2.TXT": {ThemeArchiveDocument, "LICENSE-2.TXT"},
		// A file in the templates. Whether it has the name of a template
		// is for the reader of the theme.
		"templates/home.html":  {ThemeArchiveTemplate, "home.html"},
		"templates/group.html": {ThemeArchiveTemplate, "group.html"},
		"templates/notes.txt":  {ThemeArchiveTemplate, "notes.txt"},
		// Hidden entries and what a system puts into an archive by itself.
		".DS_Store":            {ThemeArchiveIgnored, ""},
		"._theme.yml":          {ThemeArchiveIgnored, ""},
		".git/config":          {ThemeArchiveIgnored, ""},
		"assets/.paper.png":    {ThemeArchiveIgnored, ""},
		"__MACOSX/theme.yml":   {ThemeArchiveIgnored, ""},
		"__MACOSX/._theme.yml": {ThemeArchiveIgnored, ""},
		"assets/__MACOSX":      {ThemeArchiveIgnored, ""},
		".hidden/theme.yml":    {ThemeArchiveIgnored, ""},
		"assets/.cache/a.png":  {ThemeArchiveIgnored, ""},
		"templates/.home.html": {ThemeArchiveIgnored, ""},
		// No files of a theme: another name, another place, another kind.
		"":                        {ThemeArchiveOther, ""},
		"theme.yaml":              {ThemeArchiveOther, ""},
		"Theme.yml":               {ThemeArchiveOther, ""},
		"THEME.CSS":               {ThemeArchiveOther, ""},
		"theme.yml ":              {ThemeArchiveOther, ""},
		"theme.yml/":              {ThemeArchiveOther, ""},
		"theme.yml\x00":           {ThemeArchiveOther, ""},
		"sub/theme.yml":           {ThemeArchiveOther, ""},
		"sub/theme.css":           {ThemeArchiveOther, ""},
		"templates":               {ThemeArchiveOther, ""},
		"templates/":              {ThemeArchiveOther, ""},
		"templates/sub/home.html": {ThemeArchiveOther, ""},
		"Templates/home.html":     {ThemeArchiveOther, ""},
		"templates/../theme.yml":  {ThemeArchiveOther, ""},
		"templates//home.html":    {ThemeArchiveOther, ""},
		"templates\\home.html":    {ThemeArchiveOther, ""},
		"home.html":               {ThemeArchiveOther, ""},
		"assets":                  {ThemeArchiveOther, ""},
		"assets/":                 {ThemeArchiveOther, ""},
		"assets/paper.svg":        {ThemeArchiveOther, ""},
		"assets/notes.txt":        {ThemeArchiveOther, ""},
		"assets/sub/paper.png":    {ThemeArchiveOther, ""},
		"Assets/paper.png":        {ThemeArchiveOther, ""},
		"assets\\paper.png":       {ThemeArchiveOther, ""},
		"paper.png":               {ThemeArchiveOther, ""},
		"license":                 {ThemeArchiveOther, ""},
		"readme":                  {ThemeArchiveOther, ""},
		"LICENSE.html":            {ThemeArchiveOther, ""},
		"notes.md.exe":            {ThemeArchiveOther, ""},
		"my notes.txt":            {ThemeArchiveOther, ""},
		"docs/notes.md":           {ThemeArchiveOther, ""},
		"preview.png":             {ThemeArchiveOther, ""},
		// A path is not cleaned: what it would name does not matter.
		"/theme.yml":            {ThemeArchiveOther, ""},
		"./theme.yml":           {ThemeArchiveOther, ""},
		"../theme.yml":          {ThemeArchiveOther, ""},
		"a/../theme.yml":        {ThemeArchiveOther, ""},
		"assets/../theme.yml":   {ThemeArchiveOther, ""},
		"assets/./paper.png":    {ThemeArchiveOther, ""},
		"assets//paper.png":     {ThemeArchiveOther, ""},
		"assets/../../x.png":    {ThemeArchiveOther, ""},
		"..":                    {ThemeArchiveOther, ""},
		"C:/themes/theme.yml":   {ThemeArchiveOther, ""},
		"//server/theme.yml":    {ThemeArchiveOther, ""},
		"assets/paper.png/more": {ThemeArchiveOther, ""},
	} {
		if kind, name := ThemeArchiveEntry(path); kind != want.kind || name != want.name {
			t.Errorf("%q: kind %d with the name %q, want kind %d with %q", path, kind, name, want.kind, want.name)
		}
	}
}

func TestIsThemeDocumentName(t *testing.T) {
	for _, name := range []string{
		"LICENSE", "NOTICE", "COPYING", "README", "a.txt", "A.TXT", "notes.md", "README.md", "OFL-1.1.txt", "third_party.txt",
		strings.Repeat("a", 60) + ".txt",
	} {
		if !IsThemeDocumentName(name) {
			t.Errorf("%q is not taken as a text document", name)
		}
	}
	for _, name := range []string{
		"", ".txt", ".md", "txt", "md", "License", "license", "README.html", "notes.markdown", "notes.txt.zip", "a b.txt", "许可.txt",
		"-notes.txt", "_notes.txt", "../a.txt", "a/b.txt", "a.txt ", "LICENSE.", "LICENSE2",
		strings.Repeat("a", 61) + ".txt",
	} {
		if IsThemeDocumentName(name) {
			t.Errorf("%q is taken as a text document", name)
		}
	}
}

func TestIsThemeDocumentContent(t *testing.T) {
	limit := strings.Repeat("a", MaximumThemeDocumentBytes)
	for name, tc := range map[string]struct {
		content string
		want    bool
	}{
		"nothing":               {"", true},
		"text":                  {"Permission is hereby granted.\n", true},
		"text of other scripts": {"许可证\n", true},
		"as large as can be":    {limit, true},
		"one byte too large":    {limit + "a", false},
		"a NUL":                 {"text\x00more", false},
		"no UTF-8":              {"\xff\xfe\x00L", false},
		"a broken sequence":     {"caf\xc3", false},
		"the start of a PNG":    {"\x89PNG\r\n\x1a\n", false},
	} {
		if got := IsThemeDocumentContent([]byte(tc.content)); got != tc.want {
			t.Errorf("%s: %v", name, got)
		}
	}
}
