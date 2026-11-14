package model

import (
	"errors"
	"math/rand"
	"strconv"
	"strings"
	"testing"
)

// A stylesheet a theme could bring, with the constructs the check accepts.
const validThemeStylesheet = `/* Rounded tiles. Text in a comment is not read: url(http://example.test/) @import "x"; */
#page-home .app-item { border-radius: calc(var(--flare-radius) * 2); background: linear-gradient(135deg, rgb(0 0 0 / 20%), transparent); }
.app-item:hover, .app-item:focus-visible { transform: translateY(-2px) scale(1.02); transition: transform .2s cubic-bezier(.2, .8, .2, 1); }
.app-title::after { content: "url(http://example.test/) @import 'x' image-set()"; }
a[href^="https://"]:not(.app-item) { text-decoration: underline; }
.bookmark-list > li:nth-child(2n + 1):is(:hover, :focus-within) { color: color-mix(in oklab, var(--flare-primary) 80%, transparent); }
@media (prefers-reduced-motion: reduce) { .app-item { transition: none } }
@supports (backdrop-filter: blur(4px)) and selector(:has(a)) { #search { backdrop-filter: blur(4px) saturate(120%); } }
@keyframes rise { from { opacity: 0; transform: translate3d(0, 8px, 0) rotate(-1deg) } to { opacity: 1 } }
@container (min-width: 40em) { .wide { width: clamp(10rem, 50%, 20rem) } }
@layer theme { .panel { inset: 0; width: min(100%, 60ch); filter: drop-shadow(0 1px 2px hsl(0 0% 0% / .3)) } }
@property --tilt { syntax: "<angle>"; inherits: false; initial-value: 0deg }
:root { --gap: 1.5e1px; --accent: #f2b84b; --empty:; --主题: 1 }
.grid { margin: -0.5em +1.5E+2px .5px 1e3px; grid-template-columns: repeat(auto-fill, minmax(8rem, 1fr)); }
.clip { clip-path: polygon(0 0, 100% 0, 50% 100%); shape-outside: circle(50% at 50% 50%); }
.count::before { content: counter(item) ". " attr(data-label); }
<!-- .old { color: red } -->
`

func TestThemeStylesheetIsAccepted(t *testing.T) {
	for name, stylesheet := range map[string]string{
		"a stylesheet of a theme":       validThemeStylesheet,
		"empty":                         "",
		"whitespace":                    " \n\t\r\n\f ",
		"a comment":                     "/* url(a) */",
		"a comment that is not closed":  "a { color: red } /* url(a) @import 'b'; image-set('c')",
		"a comment with stars":          "/*** url(a) **/ a { color: red }",
		"a backslash in a comment":      `/* a *\/ url(a) */ a { color: red }`,
		"a byte order mark":             "\uFEFFa { color: red }",
		"line breaks of every kind":     "a {\r\n color: red;\r background: blue;\f}\n",
		"url as a name":                 ".url, #url, a[url], url { --url: 1; url: 2 }",
		"url as a unit":                 "a { width: 1url(2) }",
		"url after a number sign":       "#url(2) { color: red }",
		"url in a string":               `a { content: "url(a)" 'url(b)' }`,
		"an escaped quote in a string":  `a { content: "a\" url(b)"; color: red }`,
		"a string over two lines":       "a { content: \"a\\\nurl(b)\" }",
		"a string over two lines, CRLF": "a { content: \"a\\\r\nurl(b)\" }",
		"markup in a string":            `a { content: "</style><script>alert(1)</script>" }`,
		"a string that is not closed":   `a { content: "url(a) @import`,
		"names beyond ASCII":            ".主题 > .tête { --é: 1 } #日本語 { color: red }",
		"characters in strings":         "a { content: \"→ × ÷ \u00a0 \u2028\" } /* → × ÷ */",
		"an escape at the end":          "a { color: red } \\",
		"an at-rule with a parenthesis": "@media(min-width:1px){a{color:red}}",
		"upper case":                    "@MEDIA (MIN-WIDTH: 1PX) { A { COLOR: RGB(0 0 0); TRANSFORM: TRANSLATEX(1PX) } }",
		"a font that is installed":      "@font-face { font-family: a; src: local(Arial), local(\"Helvetica Neue\") format(\"truetype\") } a { font-family: a }",
		"escaped names":                 `@\6d edia screen { a { color: r\67 b(0 0 0); width: \63 alc(1px + 1px) } }`,
		"numbers":                       "a { margin: 1 +1 -1 1. .1 1.5 1e2 1E-2 1e+2 1e 1em -.5e-3% 0x10 1-- }",
		"arrows":                        "a { } --> <!-- <! - -> -- >",
		"at the limit":                  "a { color: red }" + strings.Repeat(" ", MaximumThemeStylesheetBytes-len("a { color: red }")),
	} {
		if err := CheckThemeStylesheet([]byte(stylesheet)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestThemeStylesheetLoadsNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		stylesheet string
		reason     error
		line       int
		found      string
	}{
		// An address, however url is written.
		"url":                      {"a { background: url(http://example.test/a.png) }", ErrThemeStyleURL, 1, "url()"},
		"relative url":             {"a { background: url(a.png) }", ErrThemeStyleURL, 1, "url()"},
		"data url":                 {"a { background: url(data:image/png;base64,AAAA) }", ErrThemeStyleURL, 1, "url()"},
		"fragment url":             {"a { filter: url(#blur) }", ErrThemeStyleURL, 1, "url()"},
		"empty url":                {"a { background: url() }", ErrThemeStyleURL, 1, "url()"},
		"quoted url":               {`a { background: url("a.png") }`, ErrThemeStyleURL, 1, "url()"},
		"single-quoted url":        {"a { background: url('a.png') }", ErrThemeStyleURL, 1, "url()"},
		"url with spaces":          {"a { background: url(   a.png   ) }", ErrThemeStyleURL, 1, "url()"},
		"url with a line break":    {"a { background: url(\n\"a.png\") }", ErrThemeStyleURL, 1, "url()"},
		"upper case":               {"a { background: URL(a.png) }", ErrThemeStyleURL, 1, "url()"},
		"mixed case":               {"a { background: uRl(a.png) }", ErrThemeStyleURL, 1, "url()"},
		"escaped first letter":     {`a { background: \75rl(a.png) }`, ErrThemeStyleURL, 1, "url()"},
		"escape with a space":      {`a { background: \75 rl(a.png) }`, ErrThemeStyleURL, 1, "url()"},
		"escape with a line break": {"a { background: \\75\nrl(a.png) }", ErrThemeStyleURL, 1, "url()"},
		"escape with a tab":        {"a { background: \\75\trl(a.png) }", ErrThemeStyleURL, 1, "url()"},
		"escape with CRLF":         {"a { background: \\75\r\nrl(a.png) }", ErrThemeStyleURL, 1, "url()"},
		"escaped middle letter":    {`a { background: u\72l(a.png) }`, ErrThemeStyleURL, 1, "url()"},
		"escaped last letter":      {`a { background: ur\6c(a.png) }`, ErrThemeStyleURL, 1, "url()"},
		"escaped upper case":       {`a { background: \55\52\4c(a.png) }`, ErrThemeStyleURL, 1, "url()"},
		"six digits":               {`a { background: \000075rl(a.png) }`, ErrThemeStyleURL, 1, "url()"},
		"six digits and a digit":   {`a { background: u\000072l(a.png) }`, ErrThemeStyleURL, 1, "url()"},
		"escaped letters":          {`a { background: \u\r\l(a.png) }`, ErrThemeStyleURL, 1, "url()"},
		"in a custom property":     {"a { --image: url(a.png) }", ErrThemeStyleURL, 1, "url()"},
		"in a condition":           {"@media screen and (min-width: url(a)) { }", ErrThemeStyleURL, 1, "url()"},
		"in a selector":            {"a[href=url(] { }", ErrThemeStyleURL, 1, "url()"},
		"at the top":               {"url(a.png)", ErrThemeStyleURL, 1, "url()"},
		"after a comment":          {"a { background: /**/url(a.png) }", ErrThemeStyleURL, 1, "url()"},
		"after two comments":       {"/* a *//* b */url(a.png)", ErrThemeStyleURL, 1, "url()"},
		"after what is no comment": {"/ * a * / url(a.png)", ErrThemeStyleURL, 1, "url()"},
		"after a comment closed behind a backslash": {`/* a \*/ url(a.png)`, ErrThemeStyleURL, 1, "url()"},
		"after a string":                            {`a { content: "a" url(a.png) }`, ErrThemeStyleURL, 1, "url()"},
		"after a string with a backslash":           {`a { content: "a\\" url(a.png) }`, ErrThemeStyleURL, 1, "url()"},
		"after a string ended by a line break":      {"a { content: \"a\n; background: url(a.png) }", ErrThemeStyleURL, 2, "url()"},
		"after a string ended by a carriage return": {"a { content: \"a\r; background: url(a.png) }", ErrThemeStyleURL, 2, "url()"},
		"after a string ended by a form feed":       {"a { content: 'a\f; background: url(a.png) }", ErrThemeStyleURL, 2, "url()"},
		"after a string over two lines":             {"a { content: \"a\\\nb\" url(a.png) }", ErrThemeStyleURL, 2, "url()"},
		"on a later line":                           {"a {\n  color: red;\r\n}\rb {\f  background: url(a.png)\n}", ErrThemeStyleURL, 5, "url()"},
		"after a byte order mark":                   {"\uFEFFa { background: url(a.png) }", ErrThemeStyleURL, 1, "url()"},

		// A function that takes an address, and any function that is not listed.
		"image-set":            {`a { background: image-set("a.png" 1x) }`, ErrThemeStyleFunction, 1, "image-set()"},
		"prefixed image-set":   {`a { background: -webkit-image-set("a.png" 1x) }`, ErrThemeStyleFunction, 1, "-webkit-image-set()"},
		"image":                {`a { background: image("a.png") }`, ErrThemeStyleFunction, 1, "image()"},
		"src":                  {`a { background: src("a.png") }`, ErrThemeStyleFunction, 1, "src()"},
		"cross-fade":           {"a { background: cross-fade(red, blue) }", ErrThemeStyleFunction, 1, "cross-fade()"},
		"element":              {"a { background: element(#a) }", ErrThemeStyleFunction, 1, "element()"},
		"paint":                {"a { background: paint(a) }", ErrThemeStyleFunction, 1, "paint()"},
		"expression":           {"a { width: expression(alert(1)) }", ErrThemeStyleFunction, 1, "expression()"},
		"type":                 {"a { width: attr(data-a type(<url>)) }", ErrThemeStyleFunction, 1, "type()"},
		"prefixed gradient":    {"a { background: -webkit-linear-gradient(red, blue) }", ErrThemeStyleFunction, 1, "-webkit-linear-gradient()"},
		"longer than url":      {"a { background: urlx(a.png) }", ErrThemeStyleFunction, 1, "urlx()"},
		"ending in url":        {"a { background: xurl(a.png) }", ErrThemeStyleFunction, 1, "xurl()"},
		"url-prefix":           {"a { background: url-prefix(a) }", ErrThemeStyleFunction, 1, "url-prefix()"},
		"escaped function":     {`a { background: \69mage-set("a.png" 1x) }`, ErrThemeStyleFunction, 1, "image-set()"},
		"upper-case function":  {`a { background: IMAGE-SET("a.png" 1x) }`, ErrThemeStyleFunction, 1, "IMAGE-SET()"},
		"unknown pseudo-class": {"a:matches(b) { }", ErrThemeStyleFunction, 1, "matches()"},
		"a letter like k":      {"a { transform: s\u212Aew(1deg) }", ErrThemeStyleFunction, 1, "s\u212Aew()"},
		"a long name":          {"a { width: " + strings.Repeat("a", 80) + "(1) }", ErrThemeStyleFunction, 1, strings.Repeat("a", 32) + "…()"},
		"inside a listed one":  {"a { width: calc(1px + image-set('a' 1x)) }", ErrThemeStyleFunction, 1, "image-set()"},

		// An at-rule that names or loads a resource, and any that is not listed.
		"import":               {`@import "a.css";`, ErrThemeStyleRule, 1, "@import"},
		"import of a url":      {"@import url(a.css);", ErrThemeStyleRule, 1, "@import"},
		"upper-case import":    {"@IMPORT 'a.css';", ErrThemeStyleRule, 1, "@IMPORT"},
		"escaped import":       {`@\69mport "a.css";`, ErrThemeStyleRule, 1, "@import"},
		"import in a block":    {"a { color: red }\n@media screen {\n  @import 'a.css';\n}", ErrThemeStyleRule, 3, "@import"},
		"namespace":            {`@namespace svg "http://www.w3.org/2000/svg";`, ErrThemeStyleRule, 1, "@namespace"},
		"charset":              {`@charset "utf-8";`, ErrThemeStyleRule, 1, "@charset"},
		"document":             {"@-moz-document url-prefix() { }", ErrThemeStyleRule, 1, "@-moz-document"},
		"a rule of the future": {"@when media(print) { }", ErrThemeStyleRule, 1, "@when"},

		// Characters browsers do not read alike, outside strings and comments.
		"multiplication sign before url": {"a { background: #\u00D7url(a.png) }", ErrThemeStyleCharacter, 1, "U+00D7"},
		"multiplication sign in a unit":  {"a { width: 1\u00D7url(a.png) }", ErrThemeStyleCharacter, 1, "U+00D7"},
		"division sign in a name":        {"a { background: x\u00F7url(a.png) }", ErrThemeStyleCharacter, 1, "U+00F7"},
		"no-break space":                 {"a {\u00A0color: red }", ErrThemeStyleCharacter, 1, "U+00A0"},
		"line separator":                 {"a { color: red }\u2028b { }", ErrThemeStyleCharacter, 1, "U+2028"},
		"arrow":                          {"a { color: red }\n\n→", ErrThemeStyleCharacter, 3, "U+2192"},
		"control character beyond ASCII": {"a { color: red } \u0085", ErrThemeStyleCharacter, 1, "U+0085"},
		"private use":                    {"a { color: \uE000 }", ErrThemeStyleCharacter, 1, "U+E000"},
	} {
		err := CheckThemeStylesheet([]byte(tc.stylesheet))
		var rejection *ThemeStyleError
		if !errors.As(err, &rejection) {
			t.Errorf("%s: accepted (%v)", name, err)
			continue
		}
		if !errors.Is(err, tc.reason) || rejection.Line != tc.line || rejection.Found != tc.found {
			t.Errorf("%s: got %v at line %d with %q, want %v at line %d with %q", name, rejection.Reason, rejection.Line, rejection.Found, tc.reason, tc.line, tc.found)
		}
		if want := "line " + strconv.Itoa(tc.line) + ": " + tc.found; rejection.Subject() != want {
			t.Errorf("%s: subject %q, want %q", name, rejection.Subject(), want)
		}
	}
}

func TestThemeStylesheetHasToBeText(t *testing.T) {
	for name, raw := range map[string][]byte{
		"a byte that is not UTF-8":      []byte("a { color: red } \xff"),
		"a truncated character":         []byte("a { content: \"\xe4\xb8\" }"),
		"a NUL":                         []byte("a { color: red }\x00"),
		"a NUL in a string":             []byte("a { content: \"\x00\" }"),
		"UTF-16 with a byte order mark": {0xFF, 0xFE, 'a', 0, ' ', 0, '{', 0, '}', 0},
		"UTF-16 the other way":          {0xFE, 0xFF, 0, 'a', 0, ' ', 0, '{', 0, '}'},
		"a surrogate":                   []byte("a { content: \"\xed\xa0\x80\" }"),
	} {
		err := CheckThemeStylesheet(raw)
		var rejection *ThemeStyleError
		if !errors.As(err, &rejection) || !errors.Is(err, ErrThemeStyleEncoding) || rejection.Line != 0 || rejection.Subject() != "" {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Every at-rule and function the check accepts is one of those that load
// nothing by themselves; a name that takes an address must never be added to
// the lists. url() is not in them: the check reads it apart and holds it to
// the assets of the theme, and @font-face has no other way to name a font.
func TestThemeStylesheetListsHoldNothingThatLoads(t *testing.T) {
	for _, name := range []string{"import", "namespace", "charset", "font-feature-values", "color-profile", "document", "-moz-document"} {
		if themeStyleRules[name] {
			t.Errorf("@%s is accepted", name)
		}
	}
	for _, name := range []string{"url", "src", "image", "image-set", "-webkit-image-set", "cross-fade", "-webkit-cross-fade", "element", "-moz-element", "paint", "url-prefix", "domain", "regexp", "expression", "symbols", "type"} {
		if themeStyleFunctions[name] {
			t.Errorf("%s() is accepted", name)
		}
	}
	for name := range themeStyleFunctions {
		if name != asciiLower(name) || strings.ContainsAny(name, " ()") {
			t.Errorf("the function %q is not written as it is compared", name)
		}
	}
	for name := range themeStyleRules {
		if name != asciiLower(name) || strings.ContainsAny(name, " @") {
			t.Errorf("the at-rule %q is not written as it is compared", name)
		}
	}
}

// The check reads any text to its end without failing, and reads it the same
// way every time. Whether what it accepts loads anything is tested against a
// browser, in tests/theme/stylesheet.browser.cjs.
func TestThemeStylesheetCheckEndsOnAnyInput(t *testing.T) {
	pieces := []string{"a", "u", "r", "l", "(", ")", "{", "}", "\"", "'", "\\", "/", "*", "@", "#", "-", "+", ".", "0", "9", "e", "E", "%",
		"<", "!", ">", ":", ";", ",", " ", "\n", "\r", "\f", "\t", "url(", "/*", "*/", "\\75", "\\\n", "@import", "@media", "calc(", "é", "×", "→", "\uFEFF", "\\0", "\\110000 ", "\\d800"}
	random := rand.New(rand.NewSource(20261115))
	for range 20000 {
		var text strings.Builder
		for range random.Intn(40) {
			text.WriteString(pieces[random.Intn(len(pieces))])
		}
		first := CheckThemeStylesheet([]byte(text.String()))
		second := CheckThemeStylesheet([]byte(text.String()))
		if (first == nil) != (second == nil) || (first != nil && first.Error() != second.Error()) {
			t.Fatalf("%q: %v, then %v", text.String(), first, second)
		}
	}
}
