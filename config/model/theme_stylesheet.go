package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// MaximumThemeStylesheetBytes is the largest stylesheet of a theme.
const MaximumThemeStylesheetBytes = 128 << 10

// Reasons the stylesheet of a theme is not accepted.
var (
	// ErrThemeStyleEncoding reports a stylesheet that is not UTF-8 text.
	ErrThemeStyleEncoding = errors.New("the stylesheet is not UTF-8 text")
	// ErrThemeStyleURL reports a url() that does not name a file in the
	// assets of the theme: a stylesheet loads nothing from anywhere else.
	ErrThemeStyleURL = errors.New("the stylesheet refers to a resource by an address")
	// ErrThemeStyleAsset reports a url() that names a file the assets of the
	// theme do not hold.
	ErrThemeStyleAsset = errors.New("the stylesheet refers to a file that is not among the assets of the theme")
	// ErrThemeStyleRule reports an at-rule a stylesheet cannot have.
	ErrThemeStyleRule = errors.New("the stylesheet has an at-rule that is not accepted")
	// ErrThemeStyleFunction reports a function a stylesheet cannot use.
	ErrThemeStyleFunction = errors.New("the stylesheet uses a function that is not accepted")
	// ErrThemeStyleCharacter reports a character outside strings and comments
	// that browsers do not read alike.
	ErrThemeStyleCharacter = errors.New("the stylesheet has a character outside strings and comments that is not accepted")
)

// ThemeStyleError reports the first part of a stylesheet that is not
// accepted: the reason, the line it is on, and what is written there, such as
// "@import" or "image-set()". Line is 0 where the reason is about the whole
// stylesheet.
type ThemeStyleError struct {
	Reason error
	Line   int
	Found  string
}

func (e *ThemeStyleError) Error() string {
	if e.Line == 0 {
		return e.Reason.Error()
	}
	return e.Reason.Error() + ": " + e.Subject()
}

func (e *ThemeStyleError) Unwrap() error { return e.Reason }

// Subject names the place for a reader: the line and what is written there.
func (e *ThemeStyleError) Subject() string {
	if e.Line == 0 {
		return ""
	}
	return "line " + strconv.Itoa(e.Line) + ": " + e.Found
}

// The at-rules a stylesheet can have. @import and @namespace name another
// resource, and @charset could have the bytes read in another encoding than
// the one they were checked in. @font-face loads a font, and is accepted
// because the only way to name one is url(), which is held to the assets of
// the theme. A rule that is not listed is not accepted, so a rule a later
// browser gives a meaning has none here.
var themeStyleRules = map[string]bool{
	"media": true, "supports": true, "container": true, "layer": true, "scope": true,
	"starting-style": true, "keyframes": true, "-webkit-keyframes": true, "property": true,
	"counter-style": true, "page": true, "font-face": true,
}

// The functions a stylesheet can use, in values and in selectors. None of them
// loads anything. url() is not among them: it is read apart from the
// functions, and held to the assets of the theme. src(), image(),
// image-set(), cross-fade(), element() and paint(), which take an address in
// other ways, are not listed, and neither is any function that is not listed,
// for the reason given for the at-rules.
var themeStyleFunctions = map[string]bool{
	// Variables and arithmetic.
	"var": true, "env": true, "attr": true, "calc": true, "min": true, "max": true, "clamp": true,
	"round": true, "mod": true, "rem": true, "abs": true, "sign": true, "sin": true, "cos": true, "tan": true,
	"asin": true, "acos": true, "atan": true, "atan2": true, "pow": true, "sqrt": true, "hypot": true,
	"log": true, "exp": true,
	// Colors.
	"rgb": true, "rgba": true, "hsl": true, "hsla": true, "hwb": true, "lab": true, "lch": true,
	"oklab": true, "oklch": true, "color": true, "color-mix": true, "light-dark": true,
	// Gradients, which are drawn and not loaded.
	"linear-gradient": true, "radial-gradient": true, "conic-gradient": true,
	"repeating-linear-gradient": true, "repeating-radial-gradient": true, "repeating-conic-gradient": true,
	// Transforms.
	"matrix": true, "matrix3d": true, "perspective": true,
	"translate": true, "translatex": true, "translatey": true, "translatez": true, "translate3d": true,
	"scale": true, "scalex": true, "scaley": true, "scalez": true, "scale3d": true,
	"rotate": true, "rotatex": true, "rotatey": true, "rotatez": true, "rotate3d": true,
	"skew": true, "skewx": true, "skewy": true,
	// Filters.
	"blur": true, "brightness": true, "contrast": true, "drop-shadow": true, "grayscale": true,
	"hue-rotate": true, "invert": true, "opacity": true, "saturate": true, "sepia": true,
	// Shapes.
	"circle": true, "ellipse": true, "inset": true, "polygon": true, "path": true, "rect": true, "xywh": true,
	// Timing.
	"cubic-bezier": true, "steps": true, "linear": true,
	// Sizes of grid tracks and boxes.
	"minmax": true, "repeat": true, "fit-content": true,
	// Counters.
	"counter": true, "counters": true,
	// Fonts: one that is installed, and the format and the technology of
	// one that is loaded.
	"local": true, "format": true, "tech": true,
	// Selectors and conditions.
	"not": true, "is": true, "where": true, "has": true, "lang": true, "dir": true,
	"nth-child": true, "nth-last-child": true, "nth-of-type": true, "nth-last-of-type": true,
	"selector": true, "style": true,
}

// CheckThemeStylesheet reports the first part of a stylesheet that a theme
// without assets cannot have, as a *ThemeStyleError, and nil for a stylesheet
// that is accepted. It is CheckThemeStylesheetFor with no file to refer to.
func CheckThemeStylesheet(raw []byte) error {
	return CheckThemeStylesheetFor(raw, nil)
}

// CheckThemeStylesheetFor reports the first part of a stylesheet that a theme
// cannot have, as a *ThemeStyleError, and nil for a stylesheet that is
// accepted. assets holds the names of the files in the assets of the theme.
//
// A stylesheet of a theme loads nothing from elsewhere: with a selector on
// the address of a link and the address of an image, a stylesheet tells
// whoever serves the image which links a page has. The check therefore works
// on the tokens of the stylesheet, read the way a browser reads them. It
// accepts a url() that names a file of the assets, written as assets/<name>,
// and no other; no at-rule and no function but the listed ones. What a
// comment or a string holds is text and is not looked at. Whether the rest is
// well-formed is left to the browser, which drops what it cannot use.
func CheckThemeStylesheetFor(raw []byte, assets map[string]bool) error {
	input, ok := cssCodePoints(raw)
	if !ok {
		return &ThemeStyleError{Reason: ErrThemeStyleEncoding}
	}
	scanner := cssScanner{input: input}
	for {
		token := scanner.next()
		var reason error
		found := token.name
		switch token.kind {
		case cssEnd:
			return nil
		case cssURL:
			// The address is given back only where it has the form of a
			// file of the assets; anything else could be any text.
			file, named := ThemeAssetReference(token.address)
			switch {
			case !token.closed || !named:
				reason, found = ErrThemeStyleURL, "url()"
			case !assets[file]:
				reason, found = ErrThemeStyleAsset, "url("+token.address+")"
			}
		case cssFunction:
			if !themeStyleFunctions[asciiLower(token.name)] {
				reason, found = ErrThemeStyleFunction, shortThemeValue(token.name)+"()"
			}
		case cssAtKeyword:
			if !themeStyleRules[asciiLower(token.name)] {
				reason, found = ErrThemeStyleRule, "@"+shortThemeValue(token.name)
			}
		case cssUncertain:
			reason, found = ErrThemeStyleCharacter, fmt.Sprintf("U+%04X", scanner.input[token.start])
		}
		if reason != nil {
			return &ThemeStyleError{Reason: reason, Line: scanner.line(token.start), Found: found}
		}
	}
}

// cssCodePoints decodes a stylesheet into code points the way a browser
// prepares it for reading: a byte order mark at the start is dropped, and a
// carriage return with or without a line feed, and a form feed, are one line
// feed each. It reports false for bytes that are not UTF-8 and for a NUL,
// which a browser would replace instead of read.
func cssCodePoints(raw []byte) ([]rune, bool) {
	if !utf8.Valid(raw) {
		return nil, false
	}
	text := strings.TrimPrefix(string(raw), "\uFEFF")
	input := make([]rune, 0, len(text))
	afterCarriageReturn := false
	for _, c := range text {
		switch c {
		case 0:
			return nil, false
		case '\r':
			input = append(input, '\n')
			afterCarriageReturn = true
			continue
		case '\n':
			if afterCarriageReturn {
				// The line feed of a carriage return and line feed pair.
				afterCarriageReturn = false
				continue
			}
		case '\f':
			c = '\n'
		}
		afterCarriageReturn = false
		input = append(input, c)
	}
	return input, true
}

// Kinds of tokens the check tells apart. Every other token is cssOther.
const (
	cssEnd = iota
	cssOther
	cssFunction
	cssAtKeyword
	cssURL
	// cssUncertain is a code point outside ASCII that some browsers take
	// into a name and others read as a token of its own.
	cssUncertain
)

// cssToken is one token of a stylesheet: its kind, where it starts, and for a
// function or an at-keyword the name with its escapes resolved. For url() it
// holds the address with its escapes resolved, and whether the address was
// written in a form a browser reads as one and closed by its parenthesis.
type cssToken struct {
	kind    int
	start   int
	name    string
	address string
	closed  bool
}

// cssScanner reads the tokens of a stylesheet as CSS Syntax Module Level 3
// defines them. It follows the tokenizer of that specification step by step,
// so that it finds the names a browser finds, also where they are written
// with escapes, in another case or behind comments. An address is read to its
// end; one that is not closed leaves the scanner where the reading stopped,
// and the check ends there.
type cssScanner struct {
	input []rune
	pos   int
}

// cssEndOfInput stands for the code point after the last one.
const cssEndOfInput = rune(-1)

// at returns the code point a number of places ahead.
func (s *cssScanner) at(ahead int) rune {
	if s.pos+ahead >= len(s.input) {
		return cssEndOfInput
	}
	return s.input[s.pos+ahead]
}

// line returns the line a position is on, counting from 1.
func (s *cssScanner) line(pos int) int {
	line := 1
	for _, c := range s.input[:pos] {
		if c == '\n' {
			line++
		}
	}
	return line
}

// next consumes one token.
func (s *cssScanner) next() cssToken {
	s.skipComments()
	token := cssToken{kind: cssOther, start: s.pos}
	c := s.at(0)
	switch {
	case c == cssEndOfInput:
		token.kind = cssEnd
	case isCSSWhitespace(c):
		for isCSSWhitespace(s.at(0)) {
			s.pos++
		}
	case c == '"' || c == '\'':
		s.pos++
		s.consumeString(c)
	case c == '#':
		s.pos++
		if isCSSNameCodePoint(s.at(0)) || isCSSEscape(s.at(0), s.at(1)) {
			s.consumeName()
		}
	case c == '+' || c == '.':
		if startsCSSNumber(c, s.at(1), s.at(2)) {
			s.consumeNumeric()
		} else {
			s.pos++
		}
	case c == '-':
		switch {
		case startsCSSNumber(c, s.at(1), s.at(2)):
			s.consumeNumeric()
		case s.at(1) == '-' && s.at(2) == '>':
			s.pos += 3
		case startsCSSName(c, s.at(1), s.at(2)):
			return s.consumeNameLike(token)
		default:
			s.pos++
		}
	case c == '<':
		if s.at(1) == '!' && s.at(2) == '-' && s.at(3) == '-' {
			s.pos += 4
		} else {
			s.pos++
		}
	case c == '@':
		s.pos++
		if startsCSSName(s.at(0), s.at(1), s.at(2)) {
			token.kind, token.name = cssAtKeyword, s.consumeName()
		}
	case c == '\\':
		if isCSSEscape(c, s.at(1)) {
			return s.consumeNameLike(token)
		}
		s.pos++
	case c >= '0' && c <= '9':
		s.consumeNumeric()
	case isCSSNameStart(c):
		return s.consumeNameLike(token)
	default:
		if c >= 0x80 {
			token.kind = cssUncertain
		}
		s.pos++
	}
	return token
}

// skipComments consumes the comments before a token. A comment that is not
// closed runs to the end of the stylesheet.
func (s *cssScanner) skipComments() {
	for s.at(0) == '/' && s.at(1) == '*' {
		s.pos += 2
		for s.at(0) != cssEndOfInput && (s.at(0) != '*' || s.at(1) != '/') {
			s.pos++
		}
		if s.at(0) != cssEndOfInput {
			s.pos += 2
		}
	}
}

// consumeString consumes a string after its opening quote.
func (s *cssScanner) consumeString(quote rune) {
	s.consumeStringValue(quote)
}

// consumeStringValue consumes a string after its opening quote and returns
// what it holds, with its escapes resolved. A string ends at its closing
// quote, at the end of the stylesheet, and at a line break that is not
// escaped: what follows such a line break is read as tokens again. The second
// result reports the closing quote.
func (s *cssScanner) consumeStringValue(quote rune) (string, bool) {
	var value []rune
	for {
		c := s.at(0)
		switch c {
		case cssEndOfInput, '\n':
			return string(value), false
		case quote:
			s.pos++
			return string(value), true
		case '\\':
			s.pos++
			switch s.at(0) {
			case cssEndOfInput:
			case '\n':
				// An escaped line break continues the string on the next line.
				s.pos++
			default:
				value = append(value, s.consumeEscape())
			}
		default:
			s.pos++
			value = append(value, c)
		}
	}
}

// consumeNameLike consumes a name and tells what it is: the start of a
// function if a parenthesis follows it, and an address if the function is
// url, in any case and however it is escaped.
func (s *cssScanner) consumeNameLike(token cssToken) cssToken {
	token.name = s.consumeName()
	if s.at(0) != '(' {
		return token
	}
	s.pos++
	token.kind = cssFunction
	if asciiLower(token.name) == "url" {
		token.kind = cssURL
		token.address, token.closed = s.consumeAddress()
	}
	return token
}

// consumeAddress consumes what follows "url(" and returns the address with
// its escapes resolved. An address is written without quotes, which makes it
// one token up to the parenthesis, or as one string in quotes. The second
// result is false for everything else: an address that is not closed, text a
// browser does not read as an address, and a string that is followed by more
// than the parenthesis.
func (s *cssScanner) consumeAddress() (string, bool) {
	s.skipWhitespace()
	if quote := s.at(0); quote == '"' || quote == '\'' {
		s.pos++
		address, closed := s.consumeStringValue(quote)
		s.skipWhitespace()
		if !closed || s.at(0) != ')' {
			return address, false
		}
		s.pos++
		return address, true
	}
	var address []rune
	for {
		c := s.at(0)
		switch {
		case c == ')':
			s.pos++
			return string(address), true
		case c == cssEndOfInput:
			return string(address), false
		case isCSSWhitespace(c):
			// Whitespace ends an address: only the parenthesis can follow.
			s.skipWhitespace()
			if s.at(0) != ')' {
				return string(address), false
			}
			s.pos++
			return string(address), true
		case c == '"' || c == '\'' || c == '(' || isCSSNonPrintable(c):
			return string(address), false
		case c == '\\':
			if !isCSSEscape(c, s.at(1)) {
				return string(address), false
			}
			s.pos++
			address = append(address, s.consumeEscape())
		default:
			s.pos++
			address = append(address, c)
		}
	}
}

// skipWhitespace consumes whitespace.
func (s *cssScanner) skipWhitespace() {
	for isCSSWhitespace(s.at(0)) {
		s.pos++
	}
}

// consumeName consumes a name: name code points and escapes.
func (s *cssScanner) consumeName() string {
	var name []rune
	for {
		c := s.at(0)
		switch {
		case isCSSNameCodePoint(c):
			s.pos++
			name = append(name, c)
		case isCSSEscape(c, s.at(1)):
			s.pos++
			name = append(name, s.consumeEscape())
		default:
			return string(name)
		}
	}
}

// consumeEscape consumes an escape after its backslash and returns the code
// point it stands for: up to six hexadecimal digits with one whitespace after
// them, or the code point that follows the backslash.
func (s *cssScanner) consumeEscape() rune {
	c := s.at(0)
	if c == cssEndOfInput {
		return utf8.RuneError
	}
	if !isCSSHexDigit(c) {
		s.pos++
		return c
	}
	value := rune(0)
	for digits := 0; digits < 6 && isCSSHexDigit(s.at(0)); digits++ {
		value = value<<4 | cssHexValue(s.at(0))
		s.pos++
	}
	if isCSSWhitespace(s.at(0)) {
		s.pos++
	}
	if value == 0 || value > utf8.MaxRune || (value >= 0xD800 && value <= 0xDFFF) {
		return utf8.RuneError
	}
	return value
}

// consumeNumeric consumes a number with what belongs to it: a unit, which is
// a name, or a percent sign.
func (s *cssScanner) consumeNumeric() {
	if c := s.at(0); c == '+' || c == '-' {
		s.pos++
	}
	s.consumeDigits()
	if s.at(0) == '.' && isCSSDigit(s.at(1)) {
		s.pos += 2
		s.consumeDigits()
	}
	if c := s.at(0); c == 'e' || c == 'E' {
		sign := s.at(1) == '+' || s.at(1) == '-'
		if isCSSDigit(s.at(1)) || (sign && isCSSDigit(s.at(2))) {
			s.pos += 2
			if sign {
				s.pos++
			}
			s.consumeDigits()
		}
	}
	switch {
	case startsCSSName(s.at(0), s.at(1), s.at(2)):
		s.consumeName()
	case s.at(0) == '%':
		s.pos++
	}
}

func (s *cssScanner) consumeDigits() {
	for isCSSDigit(s.at(0)) {
		s.pos++
	}
}

// isCSSWhitespace reports a line feed, a tab or a space. The other line
// breaks are line feeds by the time the tokens are read.
func isCSSWhitespace(c rune) bool {
	return c == '\n' || c == '\t' || c == ' '
}

func isCSSDigit(c rune) bool {
	return c >= '0' && c <= '9'
}

// isCSSNonPrintable reports a control character that cannot be written in an
// address without quotes.
func isCSSNonPrintable(c rune) bool {
	return (c >= 0 && c <= 8) || c == 0x0B || (c >= 0x0E && c <= 0x1F) || c == 0x7F
}

func isCSSHexDigit(c rune) bool {
	return isCSSDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// cssHexValue returns the value of a digit isCSSHexDigit accepted.
func cssHexValue(c rune) rune {
	switch {
	case c <= '9':
		return c - '0'
	case c >= 'a':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}

// isCSSNameStart reports a code point a name can start with: a letter, an
// underscore, or a code point outside ASCII that every reading of the
// specification takes into names.
func isCSSNameStart(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || isCSSNameBeyondASCII(c)
}

// cssNameRanges are the code points outside ASCII that are part of a name in
// the current CSS Syntax Module. Earlier versions, and browsers that follow
// them, take every code point outside ASCII into a name. For a code point in
// these ranges the two readings agree. Any other code point outside ASCII is
// read in two ways: as part of a name, or as a token that ends one, and what
// follows it could then be a name of its own, such as url. The check accepts
// such a code point in strings and comments only.
var cssNameRanges = [][2]rune{
	{0x00B7, 0x00B7}, {0x00C0, 0x00D6}, {0x00D8, 0x00F6}, {0x00F8, 0x037D}, {0x037F, 0x1FFF},
	{0x200C, 0x200D}, {0x203F, 0x2040}, {0x2070, 0x218F}, {0x2C00, 0x2FEF}, {0x3001, 0xD7FF},
	{0xF900, 0xFDCF}, {0xFDF0, 0xFFFD}, {0x10000, utf8.MaxRune},
}

func isCSSNameBeyondASCII(c rune) bool {
	for _, bounds := range cssNameRanges {
		if c >= bounds[0] && c <= bounds[1] {
			return true
		}
	}
	return false
}

// isCSSNameCodePoint reports a code point a name can continue with.
func isCSSNameCodePoint(c rune) bool {
	return isCSSNameStart(c) || isCSSDigit(c) || c == '-'
}

// isCSSEscape reports whether two code points are the start of an escape: a
// backslash that is not followed by a line break.
func isCSSEscape(first, second rune) bool {
	return first == '\\' && second != '\n'
}

// startsCSSName reports whether three code points start a name.
func startsCSSName(first, second, third rune) bool {
	switch first {
	case '-':
		return isCSSNameStart(second) || second == '-' || isCSSEscape(second, third)
	case '\\':
		return isCSSEscape(first, second)
	}
	return isCSSNameStart(first)
}

// startsCSSNumber reports whether three code points start a number.
func startsCSSNumber(first, second, third rune) bool {
	switch first {
	case '+', '-':
		return isCSSDigit(second) || (second == '.' && isCSSDigit(third))
	case '.':
		return isCSSDigit(second)
	}
	return isCSSDigit(first)
}

// asciiLower writes the ASCII letters of a name in lower case, which is how
// names of CSS are compared. Letters outside ASCII stay as they are: a name
// with one is not the name it resembles.
func asciiLower(name string) string {
	lowered := []byte(name)
	for i, c := range lowered {
		if c >= 'A' && c <= 'Z' {
			lowered[i] = c + 'a' - 'A'
		}
	}
	return string(lowered)
}
