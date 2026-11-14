package model

import (
	"errors"
	"strings"
	"testing"
)

func TestThemeAssetName(t *testing.T) {
	for _, name := range []string{"a.png", "Logo-2_x.final.woff2", "0.gif", "a", "A.PNG", "a..b", "a-", strings.Repeat("a", 60) + ".png"} {
		if !IsThemeAssetName(name) {
			t.Errorf("%q is not accepted", name)
		}
	}
	for _, name := range []string{
		"", ".", "..", ".hidden.png", "-a.png", "_a.png", "a b.png", "a/b.png", `a\b.png`, "a%2e.png", "a?.png", "a#.png",
		"a:b.png", "a\x00.png", "a\n.png", "图.png", "é.png", "a.png ", " a.png", strings.Repeat("a", 61) + ".png",
	} {
		if IsThemeAssetName(name) {
			t.Errorf("%q is accepted", name)
		}
	}
}

func TestThemeAssetKinds(t *testing.T) {
	// How files of each kind start, with the type they are served as and the
	// format they are in. Two names can be one format.
	kinds := map[string][3]string{
		"a.png":  {"image/png", "png", "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"},
		"a.jpg":  {"image/jpeg", "jpeg", "\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01"},
		"a.jpeg": {"image/jpeg", "jpeg", "\xff\xd8\xff\xe1\x00\x10Exif\x00\x00"},
		"a.gif":  {"image/gif", "gif", "GIF89a\x01\x00\x01\x00\x80\x00"},
		"b.gif":  {"image/gif", "gif", "GIF87a\x01\x00\x01\x00\x80\x00"},
		"a.webp": {"image/webp", "webp", "RIFF\x24\x00\x00\x00WEBPVP8 "},
		"a.avif": {"image/avif", "avif", "\x00\x00\x00\x1cftypavif\x00\x00\x00\x00avifmif1miaf"},
		"b.avif": {"image/avif", "avif", "\x00\x00\x00\x1cftypavis\x00\x00\x00\x00avismsf1miaf"},
		// An encoder can name a more general brand first, and the one of
		// the format among those the file is compatible with.
		"c.avif":  {"image/avif", "avif", "\x00\x00\x00\x20ftypmif1\x00\x00\x00\x00mif1avifmiafMA1B"},
		"a.woff2": {"font/woff2", "woff2", "wOF2\x00\x01\x00\x00\x00\x00\x00\x00"},
		"a.woff":  {"font/woff", "woff", "wOFF\x00\x01\x00\x00\x00\x00\x00\x00"},
		// TrueType and OpenType fonts share a container, and both
		// extensions are in use for both.
		"a.ttf": {"font/ttf", "font", "\x00\x01\x00\x00\x00\x0c\x00\x80\x00\x03\x00\x40"},
		"b.ttf": {"font/ttf", "font", "true\x00\x0c\x00\x80\x00\x03\x00\x40"},
		"c.ttf": {"font/ttf", "font", "OTTO\x00\x0c\x00\x80\x00\x03\x00\x40"},
		"a.otf": {"font/otf", "font", "OTTO\x00\x0c\x00\x80\x00\x03\x00\x40"},
		"b.otf": {"font/otf", "font", "\x00\x01\x00\x00\x00\x0c\x00\x80\x00\x03\x00\x40"},
	}
	for name, kind := range kinds {
		served, known := ThemeAssetType(name)
		if !known || served != kind[0] {
			t.Errorf("%s: type %q %v", name, served, known)
		}
		if !IsThemeAssetContent(name, []byte(kind[2])) {
			t.Errorf("%s: its content is not accepted", name)
		}
		// The extension is read in any case.
		upper := strings.ToUpper(name[:1]) + "." + strings.ToUpper(name[strings.LastIndexByte(name, '.')+1:])
		if served, known = ThemeAssetType(upper); !known || served != kind[0] {
			t.Errorf("%s: type %q %v", upper, served, known)
		}
		if !IsThemeAssetContent(upper, []byte(kind[2])) {
			t.Errorf("%s: its content is not accepted", upper)
		}
		// The content of another format is not that of this one.
		for other, otherKind := range kinds {
			if otherKind[1] != kind[1] && IsThemeAssetContent(name, []byte(otherKind[2])) {
				t.Errorf("%s accepts the content of %s", name, other)
			}
		}
		for _, wrong := range []string{"", kind[2][:2], "<!doctype html><script>alert(1)</script>", "<svg xmlns=\"http://www.w3.org/2000/svg\">", "#!/bin/sh\n"} {
			if IsThemeAssetContent(name, []byte(wrong)) {
				t.Errorf("%s accepts %q", name, wrong)
			}
		}
	}
	// A file of the same family of formats that is not an AVIF image, and
	// file type boxes that do not hold what they say.
	for name, start := range map[string]string{
		"an image of another codec":       "\x00\x00\x00\x18ftypheic\x00\x00\x00\x00mif1heic",
		"a video":                         "\x00\x00\x00\x20ftypisom\x00\x00\x02\x00isomiso2avc1mp41",
		"the brand in the version":        "\x00\x00\x00\x14ftypmif1avifmif1",
		"the brand after the box":         "\x00\x00\x00\x14ftypmif1\x00\x00\x00\x00mif1avif",
		"a box shorter than its brand":    "\x00\x00\x00\x08ftypavif\x00\x00\x00\x00avif",
		"a box without a length":          "\x00\x00\x00\x00ftypavif\x00\x00\x00\x00avif",
		"another box":                     "\x00\x00\x00\x1cfreeavif\x00\x00\x00\x00avif",
		"a brand that is not on a border": "\x00\x00\x00\x1cftypmif1\x00\x00\x00\x00miavif1\x00",
	} {
		if IsThemeAssetContent("a.avif", []byte(start)) {
			t.Errorf("%s is accepted as an AVIF image", name)
		}
	}
	// The brands are read as far as the start of a file holds them.
	long := "\x00\x00\x01\x00ftypmif1\x00\x00\x00\x00" + strings.Repeat("miaf", 11) + "avif"
	if len(long) != ThemeAssetHeadBytes || !IsThemeAssetContent("a.avif", []byte(long)) {
		t.Errorf("a brand at the end of the %d bytes that are looked at is not found", len(long))
	}
	if IsThemeAssetContent("a.avif", []byte(long[:len(long)-1])) {
		t.Error("a brand that is cut off is found")
	}
	// A theme brings pictures and fonts, and nothing that can carry a script
	// or name another file.
	for _, name := range []string{"a.svg", "a.svgz", "a.html", "a.htm", "a.js", "a.mjs", "a.css", "a.json", "a.xml", "a.ico", "a.bmp", "a.tiff", "a.pdf", "a.eot", "a.wasm", "a.txt", "a.yml", "a.heic", "a.mp4", "a.ttc", "a", "a.", "png", ".png", "a.png.html", "a.png.", "a.png "} {
		if served, known := ThemeAssetType(name); known {
			t.Errorf("%s is served as %s", name, served)
		}
		if IsThemeAssetContent(name, []byte(kinds["a.png"][2])) {
			t.Errorf("%s: content accepted", name)
		}
	}
}

// A file of the assets is meant as an asset by its extension alone, so that a
// file with a name that is not accepted is reported instead of left alone.
func TestThemeAssetExtension(t *testing.T) {
	for _, name := range []string{"a.png", "A.PNG", "my photo.JPeG", ".png", "a.b.woff2", "图.gif", "a/b.otf", "a.png.webp"} {
		if !HasThemeAssetExtension(name) {
			t.Errorf("%q is not meant as an asset", name)
		}
	}
	for _, name := range []string{"", "png", "a", "a.", "a.png.", "a.png ", "a.svg", "a.txt", "theme.css", "theme.yml", "@eaDir", "Thumbs.db", ".DS_Store", "a.png.html", "a.pngx", "a.xpng"} {
		if HasThemeAssetExtension(name) {
			t.Errorf("%q is meant as an asset", name)
		}
	}
}

func TestThemeAssetReference(t *testing.T) {
	for address, name := range map[string]string{
		"assets/a.png":                "a.png",
		"assets/Logo-2_x.final.woff2": "Logo-2_x.final.woff2",
		"assets/a":                    "a",
	} {
		if got, named := ThemeAssetReference(address); !named || got != name {
			t.Errorf("%q: %q %v", address, got, named)
		}
	}
	for _, address := range []string{
		"", "a.png", "assets", "assets/", "/assets/a.png", "./assets/a.png", "../assets/a.png", "assets//a.png",
		"assets/./a.png", "assets/../a.png", "assets/..", "assets/a/b.png", "assets/a/../b.png", `assets\a.png`, `assets/..\a.png`,
		"assets/a.png?x=1", "assets/a.png#f", "assets/a.png/", "ASSETS/a.png", "Assets/a.png", "assets/.a.png",
		"http://example.test/assets/a.png", "https://example.test/assets/a.png", "//example.test/assets/a.png",
		"data:image/png;base64,AAAA", "javascript:alert(1)", "file:///assets/a.png", "assets:a.png",
		"assets/a%2epng", "assets/%2e%2e/a.png", "assets/a.png ", " assets/a.png", "assets/ a.png", "assets/a\t.png",
		"assets/a\n.png", "assets/a\x00.png", "assets/图.png", "assets/" + strings.Repeat("a", 61) + ".png",
	} {
		if name, named := ThemeAssetReference(address); named {
			t.Errorf("%q names %q", address, name)
		}
	}
}

func TestThemeStylesheetRefersToItsAssets(t *testing.T) {
	assets := map[string]bool{"a.png": true, "b.jpg": true, "font.woff2": true}
	for name, stylesheet := range map[string]string{
		"an address":                "a { background: url(assets/a.png) }",
		"in double quotes":          `a { background: url("assets/a.png") }`,
		"in single quotes":          "a { background: url('assets/a.png') }",
		"with spaces":               "a { background: url(   assets/a.png   ) }",
		"with spaces and quotes":    "a { background: url(  \"assets/a.png\"\t) }",
		"with line breaks":          "a { background: url(\n  assets/a.png\r\n) }",
		"upper case":                "a { background: URL(assets/a.png) }",
		"an escaped url":            `a { background: \75rl(assets/a.png) }`,
		"an escaped address":        `a { background: url(assets/\61.png) }`,
		"an escaped dot":            `a { background: url(assets/a\.png) }`,
		"an escape in quotes":       `a { background: url("assets/\61 .png") }`,
		"with the rest of a value":  "a { background: #000 url(assets/a.png) no-repeat center / cover, url('assets/b.jpg') }",
		"a font":                    `@font-face { font-family: brought; src: url(assets/font.woff2) format("woff2") tech(variations), local(Arial) } a { font-family: brought }`,
		"in a custom property":      "a { --image: url(assets/a.png); background: var(--image) }",
		"in a condition":            "@media (min-width: 40em) { a { background: url(assets/a.png) } }",
		"twice":                     "a { background: url(assets/a.png) } b { background: url(assets/b.jpg) }",
		"at the end":                "a { background: url(assets/a.png)",
		"in a comment and a string": `/* url(http://example.test/) */ a { content: "url(http://example.test/)"; background: url(assets/a.png) }`,
	} {
		if err := CheckThemeStylesheetFor([]byte(stylesheet), assets); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		// Without assets the same stylesheet names a file that is not there.
		if err := CheckThemeStylesheet([]byte(stylesheet)); !errors.Is(err, ErrThemeStyleAsset) {
			t.Errorf("%s, without assets: %v", name, err)
		}
	}

	for name, tc := range map[string]struct {
		stylesheet string
		reason     error
		line       int
		found      string
	}{
		// A file the assets do not hold.
		"a missing file":          {"a { background: url(assets/c.png) }", ErrThemeStyleAsset, 1, "url(assets/c.png)"},
		"another case":            {"a { background: url(assets/A.png) }", ErrThemeStyleAsset, 1, "url(assets/A.png)"},
		"a missing file, later":   {"a { background: url(assets/a.png) }\nb {\n  background: url('assets/missing.gif')\n}", ErrThemeStyleAsset, 3, "url(assets/missing.gif)"},
		"an escaped missing file": {`a { background: url(assets/\63.png) }`, ErrThemeStyleAsset, 1, "url(assets/c.png)"},

		// Everything that is not assets/<name>.
		"another origin":             {"a { background: url(https://example.test/assets/a.png) }", ErrThemeStyleURL, 1, "url()"},
		"another origin, no scheme":  {"a { background: url(//example.test/assets/a.png) }", ErrThemeStyleURL, 1, "url()"},
		"from the root":              {"a { background: url(/assets/a.png) }", ErrThemeStyleURL, 1, "url()"},
		"from the root of a theme":   {"a { background: url(/theme/other/0123456789abcdef/assets/a.png) }", ErrThemeStyleURL, 1, "url()"},
		"the directory above":        {"a { background: url(assets/../a.png) }", ErrThemeStyleURL, 1, "url()"},
		"the directory above, first": {"a { background: url(../other/assets/a.png) }", ErrThemeStyleURL, 1, "url()"},
		"a directory below":          {"a { background: url(assets/images/a.png) }", ErrThemeStyleURL, 1, "url()"},
		"this directory":             {"a { background: url(./assets/a.png) }", ErrThemeStyleURL, 1, "url()"},
		"not in assets":              {"a { background: url(a.png) }", ErrThemeStyleURL, 1, "url()"},
		"the stylesheet itself":      {"a { background: url(theme.css) }", ErrThemeStyleURL, 1, "url()"},
		"a backslash":                {`a { background: url("assets\a.png") }`, ErrThemeStyleURL, 1, "url()"},
		"an escaped backslash":       {`a { background: url(assets\\..\\a.png) }`, ErrThemeStyleURL, 1, "url()"},
		"an escaped slash":           {`a { background: url(assets/\2e\2e/a.png) }`, ErrThemeStyleURL, 1, "url()"},
		"percent encoding":           {"a { background: url(assets/%2e%2e/a.png) }", ErrThemeStyleURL, 1, "url()"},
		"a query":                    {"a { background: url(assets/a.png?v=2) }", ErrThemeStyleURL, 1, "url()"},
		"a fragment":                 {"a { background: url(assets/a.png#part) }", ErrThemeStyleURL, 1, "url()"},
		"upper-case directory":       {"a { background: url(ASSETS/a.png) }", ErrThemeStyleURL, 1, "url()"},
		"data":                       {"a { background: url(data:image/png;base64,AAAA) }", ErrThemeStyleURL, 1, "url()"},
		"empty":                      {"a { background: url() }", ErrThemeStyleURL, 1, "url()"},
		"empty string":               {`a { background: url("") }`, ErrThemeStyleURL, 1, "url()"},
		"an escaped tab":             {`a { background: url(assets/a\9 .png) }`, ErrThemeStyleURL, 1, "url()"},
		"an escaped line break":      {`a { background: url("assets/a\a .png") }`, ErrThemeStyleURL, 1, "url()"},
		"a comment in an address":    {"a { background: url(assets/a.png/**/) }", ErrThemeStyleURL, 1, "url()"},
		"a comment before":           {"a { background: url(/**/assets/a.png) }", ErrThemeStyleURL, 1, "url()"},

		// Text a browser does not read as an address, or not as this one.
		"two words":                                   {"a { background: url(assets/a.png b) }", ErrThemeStyleURL, 1, "url()"},
		"a string and a word":                         {`a { background: url("assets/a.png" b) }`, ErrThemeStyleURL, 1, "url()"},
		"two strings":                                 {`a { background: url("assets/a.png" "assets/b.jpg") }`, ErrThemeStyleURL, 1, "url()"},
		"a string and a comment":                      {`a { background: url("assets/a.png" /**/) }`, ErrThemeStyleURL, 1, "url()"},
		"a quote in an address":                       {`a { background: url(assets/a.png"b) }`, ErrThemeStyleURL, 1, "url()"},
		"a parenthesis":                               {"a { background: url(assets/a(.png) }", ErrThemeStyleURL, 1, "url()"},
		"a control character":                         {"a { background: url(assets/a\x01.png) }", ErrThemeStyleURL, 1, "url()"},
		"a string that is not closed":                 {`a { background: url("assets/a.png) }`, ErrThemeStyleURL, 1, "url()"},
		"a string ended by a line break":              {"a { background: url(\"assets/a.png\n\") }", ErrThemeStyleURL, 1, "url()"},
		"a string ended by a line break, then closed": {"a { background: url(\"assets/a.png\n) }", ErrThemeStyleURL, 1, "url()"},
		"other quotes":                                {`a { background: url('assets/a.png") }`, ErrThemeStyleURL, 1, "url()"},
		"not closed":                                  {"a { background: url(assets/a.png", ErrThemeStyleURL, 1, "url()"},
		"not closed after a space":                    {"a { background: url(assets/a.png ", ErrThemeStyleURL, 1, "url()"},
		"a backslash at the end":                      {"a { background: url(assets/a.png\\\n) }", ErrThemeStyleURL, 1, "url()"},

		// A stylesheet is read on after an address that is accepted.
		"a second address":    {"a { background: url(assets/a.png) }\nb { background: url(https://example.test/a.png) }", ErrThemeStyleURL, 2, "url()"},
		"an at-rule after":    {"a { background: url(assets/a.png) }\n\n@import \"a.css\";", ErrThemeStyleRule, 3, "@import"},
		"a function after":    {"a { background: url(assets/a.png), image-set(\"assets/b.jpg\" 1x) }", ErrThemeStyleFunction, 1, "image-set()"},
		"the same in a set":   {"a { background: image-set(url(assets/a.png) 1x) }", ErrThemeStyleFunction, 1, "image-set()"},
		"an address in src":   {`a { background: src("assets/a.png") }`, ErrThemeStyleFunction, 1, "src()"},
		"an import of a file": {"@import url(assets/a.png);", ErrThemeStyleRule, 1, "@import"},
	} {
		err := CheckThemeStylesheetFor([]byte(tc.stylesheet), assets)
		var rejection *ThemeStyleError
		if !errors.As(err, &rejection) {
			t.Errorf("%s: accepted (%v)", name, err)
			continue
		}
		if !errors.Is(err, tc.reason) || rejection.Line != tc.line || rejection.Found != tc.found {
			t.Errorf("%s: got %v at line %d with %q, want %v at line %d with %q", name, rejection.Reason, rejection.Line, rejection.Found, tc.reason, tc.line, tc.found)
		}
	}
}
