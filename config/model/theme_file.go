package model

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// ThemeContract is the version of the theme file format this release reads.
const ThemeContract = 1

// Names of the style tokens a theme file can set. Stylesheets use them with
// the prefix --flare-; docs/theme-tokens.md describes what each one is for.
const (
	TokenBackground = "background"
	TokenPrimary    = "primary"
	TokenAccent     = "accent"
	TokenIcon       = "icon"
	TokenMuted      = "muted"
	TokenBorder     = "border"
	TokenFocus      = "focus"
	TokenInverse    = "inverse"
	TokenHover      = "hover"
	TokenRadius     = "radius"
	TokenFontFamily = "font-family"
)

// ThemeTokenNames lists the tokens in the order of their documentation. The
// first three are the colors every theme has to give.
var ThemeTokenNames = []string{
	TokenBackground, TokenPrimary, TokenAccent,
	TokenIcon, TokenMuted, TokenBorder, TokenFocus, TokenInverse,
	TokenHover, TokenRadius, TokenFontFamily,
}

// MaximumThemeRadius is the largest value of the radius token, in pixels.
// Stylesheets use up to three times the token for larger corners.
const MaximumThemeRadius = 32

// maximumFontFamilyLength bounds the font-family token.
const maximumFontFamilyLength = 160

var (
	// ErrThemeSyntax reports a theme file that is not the expected YAML document.
	ErrThemeSyntax = errors.New("theme file cannot be read")
	// ErrThemeContract reports a theme file written for another version of the format.
	ErrThemeContract = errors.New("theme file is written for another contract version")
	// ErrThemeToken reports a token name that this release does not know.
	ErrThemeToken = errors.New("theme file sets an unknown token")
	// ErrThemeTokenValue reports a token value outside the accepted form.
	ErrThemeTokenValue = errors.New("theme token has a value that is not accepted")
	// ErrThemeText reports a name, version, author or license that is not plain text of the accepted length.
	ErrThemeText = errors.New("theme file has a text that is not accepted")
)

// ThemeFileError is the reason a theme file is rejected. Reason is one of the
// errors above or one of the palette errors. Subject names the part of the
// file the reason is about, a token or a key, or holds the message of the
// YAML parser; it can be empty.
type ThemeFileError struct {
	Reason  error
	Subject string
}

func (e *ThemeFileError) Error() string {
	if e.Subject == "" {
		return e.Reason.Error()
	}
	return e.Reason.Error() + ": " + e.Subject
}

func (e *ThemeFileError) Unwrap() error { return e.Reason }

func themeFileError(reason error, subject string) error {
	return &ThemeFileError{Reason: reason, Subject: subject}
}

// Longest accepted texts of a theme file, in characters.
const (
	maximumThemeNameLength = 48
	maximumThemeTextLength = 64
)

// ThemeFile is a theme as written in a file. Its token values end up in a
// style attribute without further escaping, so nothing is used before
// Validate has accepted it.
type ThemeFile struct {
	Contract int
	// Name is shown instead of the file name. The remaining texts describe
	// the theme and have no effect.
	Name    ThemeText
	Version string
	Author  string
	License string
	Tokens  map[string]string
	// Modes holds the token values that replace those of Tokens in light
	// and in dark mode, keyed by mode.
	Modes map[string]map[string]string
}

// ThemeText is a text of a theme file: one string, or one string per
// interface language. Language codes are kept in lower case with hyphens, so
// zh-CN, zh_CN and zh-cn name the same language.
type ThemeText struct {
	Plain     string
	Localized map[string]string
}

// UnmarshalYAML accepts a string or a mapping of language to string.
func (t *ThemeText) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode && node.Alias != nil {
		node = node.Alias
	}
	if node.Kind != yaml.MappingNode {
		return node.Decode(&t.Plain)
	}
	var written map[string]string
	if err := node.Decode(&written); err != nil {
		return err
	}
	t.Localized = make(map[string]string, len(written))
	for language, text := range written {
		code := strings.ReplaceAll(strings.ToLower(language), "_", "-")
		if _, repeated := t.Localized[code]; repeated {
			return fmt.Errorf("language %q is given more than once", code)
		}
		t.Localized[code] = text
	}
	return nil
}

// For returns the text for an interface language: the text in that language,
// the plain text, English, and then any other language.
func (t ThemeText) For(locale string) string {
	if text := t.inLanguage(locale); text != "" {
		return text
	}
	if t.Plain != "" {
		return t.Plain
	}
	if text := t.inLanguage("en"); text != "" {
		return text
	}
	for _, language := range t.languages() {
		if text := t.Localized[language]; text != "" {
			return text
		}
	}
	return ""
}

// inLanguage returns the text written for a language, or else for the first
// regional form of it: zh-cn answers for zh.
func (t ThemeText) inLanguage(language string) string {
	if language == "" {
		return ""
	}
	if text := t.Localized[language]; text != "" {
		return text
	}
	for _, code := range t.languages() {
		if text := t.Localized[code]; text != "" && strings.HasPrefix(code, language+"-") {
			return text
		}
	}
	return ""
}

// languages returns the language codes in a fixed order.
func (t ThemeText) languages() []string {
	languages := make([]string, 0, len(t.Localized))
	for language := range t.Localized {
		languages = append(languages, language)
	}
	slices.Sort(languages)
	return languages
}

// ThemeFileUnused lists the parts of a theme file that were read but not
// used, as paths in the file. Keys holds keys and modes this release does not
// know, such as layout or modes.sepia; Tokens holds tokens it does not know,
// such as tokens.shadow or modes.light.shadow. A file written for a later
// release remains usable without them.
type ThemeFileUnused struct {
	Keys   []string
	Tokens []string
}

// Empty reports whether every part of the file was used.
func (u ThemeFileUnused) Empty() bool { return len(u.Keys) == 0 && len(u.Tokens) == 0 }

// ReadThemeFile reads and validates a theme file. Keys, modes and tokens this
// release does not know are left out and reported; everything that is used
// has to be valid, or the file is rejected as a whole. A rejection is a
// *ThemeFileError.
func ReadThemeFile(raw []byte) (ThemeFile, ThemeFileUnused, error) {
	// A mapping of nodes rejects a document that is not a mapping and one
	// that repeats a key.
	var document map[string]yaml.Node
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return ThemeFile{}, ThemeFileUnused{}, themeFileError(ErrThemeSyntax, err.Error())
	}
	if document == nil {
		return ThemeFile{}, ThemeFileUnused{}, themeFileError(ErrThemeSyntax, "the file is empty")
	}
	keys := make([]string, 0, len(document))
	for key := range document {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var file ThemeFile
	var unused ThemeFileUnused
	for _, key := range keys {
		var target any
		switch key {
		case "contract":
			target = &file.Contract
		case "name":
			target = &file.Name
		case "version":
			target = &file.Version
		case "author":
			target = &file.Author
		case "license":
			target = &file.License
		case "tokens":
			target = &file.Tokens
		case "modes":
			target = &file.Modes
		default:
			unused.Keys = append(unused.Keys, key)
			continue
		}
		node := document[key]
		if err := node.Decode(target); err != nil {
			return ThemeFile{}, ThemeFileUnused{}, themeFileError(ErrThemeSyntax, key+": "+err.Error())
		}
	}
	unused.Tokens = dropUnknownTokens(file.Tokens, "tokens.", unused.Tokens)
	for mode, replaced := range file.Modes {
		if mode != ThemeModeLight && mode != ThemeModeDark {
			unused.Keys = append(unused.Keys, "modes."+mode)
			delete(file.Modes, mode)
			continue
		}
		unused.Tokens = dropUnknownTokens(replaced, "modes."+mode+".", unused.Tokens)
	}
	slices.Sort(unused.Keys)
	slices.Sort(unused.Tokens)
	if err := file.Validate(); err != nil {
		return ThemeFile{}, unused, err
	}
	return file, unused, nil
}

// dropUnknownTokens removes the tokens this release does not know from one
// mapping of a theme file and adds their paths to the list.
func dropUnknownTokens(tokens map[string]string, path string, unused []string) []string {
	for token := range tokens {
		if _, known := themeTokenChecks[token]; !known {
			unused = append(unused, path+token)
			delete(tokens, token)
		}
	}
	return unused
}

// ParseThemeFile reads a theme file that has to be used in full, as the
// bundled themes are. An unknown key or token is an error.
func ParseThemeFile(raw []byte) (ThemeFile, error) {
	file, unused, err := ReadThemeFile(raw)
	if err != nil {
		return ThemeFile{}, err
	}
	if len(unused.Keys) > 0 {
		return ThemeFile{}, themeFileError(ErrThemeSyntax, "unknown key "+strconv.Quote(unused.Keys[0]))
	}
	if len(unused.Tokens) > 0 {
		return ThemeFile{}, themeFileError(ErrThemeToken, unused.Tokens[0])
	}
	return file, nil
}

// Validate accepts a theme whose tokens can be written into a page. The three
// colors are required and have to pass the checks of a custom palette; the
// other tokens are optional. The values of a mode replace those of the theme,
// and the result has to pass the same checks. The first reason found is
// returned as a *ThemeFileError that names the part of the file, and the
// order of the checks does not vary.
func (f ThemeFile) Validate() error {
	if f.Contract != ThemeContract {
		return themeFileError(ErrThemeContract, strconv.Itoa(f.Contract))
	}
	if err := validateThemeTokens(f.Tokens, "tokens."); err != nil {
		return err
	}
	for _, name := range ThemeTokenNames[:3] {
		if _, set := f.Tokens[name]; !set {
			return themeFileError(ErrPaletteColor, "tokens."+name)
		}
	}
	if err := f.Palette().Validate(); err != nil {
		return themeFileError(err, "")
	}
	modes := make([]string, 0, len(f.Modes))
	for mode := range f.Modes {
		modes = append(modes, mode)
	}
	slices.Sort(modes)
	for _, mode := range modes {
		if mode != ThemeModeLight && mode != ThemeModeDark {
			return themeFileError(ErrThemeSyntax, "unknown mode "+strconv.Quote(mode))
		}
		if err := validateThemeTokens(f.Modes[mode], "modes."+mode+"."); err != nil {
			return err
		}
		if err := f.Theme("").In(mode).Colors.Validate(); err != nil {
			return themeFileError(err, "modes."+mode)
		}
	}
	if !isThemeText(f.Name.Plain, maximumThemeNameLength) {
		return themeFileError(ErrThemeText, "name")
	}
	for _, language := range f.Name.languages() {
		if !isThemeWord(language) || !isThemeText(f.Name.Localized[language], maximumThemeNameLength) {
			return themeFileError(ErrThemeText, "name: "+strconv.Quote(language))
		}
	}
	for _, text := range []struct{ key, value string }{{"version", f.Version}, {"author", f.Author}, {"license", f.License}} {
		if !isThemeText(text.value, maximumThemeTextLength) {
			return themeFileError(ErrThemeText, text.key)
		}
	}
	return nil
}

// validateThemeTokens checks one mapping of tokens: the tokens of the theme
// or the tokens of a mode. An unknown token is reported before a value, the
// first in the order of the names; values are checked in the order of the
// documentation.
func validateThemeTokens(tokens map[string]string, path string) error {
	unknown := make([]string, 0)
	for name := range tokens {
		if _, known := themeTokenChecks[name]; !known {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		return themeFileError(ErrThemeToken, path+slices.Min(unknown))
	}
	for _, name := range ThemeTokenNames {
		if value, set := tokens[name]; set && !themeTokenChecks[name](value) {
			return themeFileError(ErrThemeTokenValue, path+name)
		}
	}
	return nil
}

// isThemeText accepts an empty text or plain text of a limited length. Texts
// are shown on the settings page, where they are escaped; control characters
// and surrounding spaces are rejected because they have no place in a label.
func isThemeText(text string, limit int) bool {
	if text == "" {
		return true
	}
	if !utf8.ValidString(text) || utf8.RuneCountInString(text) > limit || strings.TrimSpace(text) != text {
		return false
	}
	for _, r := range text {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// isThemeWord accepts a short lower-case identifier such as a language code
// or a theme name: letters and digits, with single hyphens between them.
func isThemeWord(word string) bool {
	if word == "" || len(word) > maximumThemeWordLength {
		return false
	}
	for i := 0; i < len(word); i++ {
		c := word[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c == '-' && i > 0 && i < len(word)-1 && word[i-1] != '-':
		default:
			return false
		}
	}
	return true
}

// maximumThemeWordLength bounds theme names and language codes.
const maximumThemeWordLength = 32

// IsThemeName reports whether a name can identify a theme file: lower-case
// letters and digits with single hyphens between them, at most 32 bytes.
func IsThemeName(name string) bool { return isThemeWord(name) }

// Palette returns the three colors of the theme as written.
func (f ThemeFile) Palette() Palette {
	return Palette{Background: f.Tokens[TokenBackground], Primary: f.Tokens[TokenPrimary], Accent: f.Tokens[TokenAccent]}
}

// Theme converts a validated file into the theme the application uses.
func (f ThemeFile) Theme(name string) Theme {
	theme := Theme{Name: name, Colors: f.Palette(), Title: f.Name, Version: f.Version, Author: f.Author, License: f.License}
	for _, token := range ThemeTokenNames[3:] {
		if value, set := f.Tokens[token]; set {
			if theme.Tokens == nil {
				theme.Tokens = make(map[string]string)
			}
			theme.Tokens[token] = value
		}
	}
	for _, mode := range []string{ThemeModeLight, ThemeModeDark} {
		if replaced := f.Modes[mode]; len(replaced) > 0 {
			if theme.Modes == nil {
				theme.Modes = make(map[string]map[string]string)
			}
			theme.Modes[mode] = maps.Clone(replaced)
		}
	}
	return theme
}

var themeTokenChecks = map[string]func(string) bool{
	TokenBackground: IsHexColor,
	TokenPrimary:    IsHexColor,
	TokenAccent:     IsHexColor,
	TokenIcon:       IsHexColor,
	TokenMuted:      IsHexColor,
	TokenBorder:     IsHexColor,
	TokenFocus:      IsHexColor,
	TokenInverse:    IsHexColor,
	TokenHover:      isOverlayColor,
	TokenRadius:     isThemeRadius,
	TokenFontFamily: isFontFamilyList,
}

// isOverlayColor accepts #RRGGBB and #RRGGBBAA. The hover token is drawn over
// the page background, so it may be translucent.
func isOverlayColor(value string) bool {
	if len(value) == 9 {
		return IsHexColor(value[:7]) && isHexPair(value[7:])
	}
	return IsHexColor(value)
}

func isHexPair(value string) bool {
	if len(value) != 2 {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// isThemeRadius accepts 0 and whole pixel lengths up to MaximumThemeRadius.
func isThemeRadius(value string) bool {
	if value == "0" {
		return true
	}
	digits, found := strings.CutSuffix(value, "px")
	if !found || digits == "" || len(digits) > 2 || (len(digits) > 1 && digits[0] == '0') {
		return false
	}
	pixels, err := strconv.Atoi(digits)
	return err == nil && pixels >= 0 && pixels <= MaximumThemeRadius
}

// isFontFamilyList accepts family names separated by commas, such as
// "Helvetica Neue, -apple-system, sans-serif". A name is one or more words
// separated by single spaces, which makes it valid CSS without quotes. Quotes
// and all other punctuation are rejected: the value is written into a style
// attribute.
func isFontFamilyList(value string) bool {
	if value == "" || len(value) > maximumFontFamilyLength {
		return false
	}
	for _, family := range strings.Split(value, ",") {
		family = strings.Trim(family, " ")
		if family == "" {
			return false
		}
		for _, word := range strings.Split(family, " ") {
			if !isFontFamilyWord(word) {
				return false
			}
		}
	}
	return true
}

// isFontFamilyWord accepts a CSS identifier made of letters, digits, hyphens
// and underscores: it starts with a letter, or with one hyphen and a letter.
func isFontFamilyWord(word string) bool {
	word = strings.TrimPrefix(word, "-")
	if word == "" {
		return false
	}
	for i, r := range word {
		switch {
		case unicode.IsLetter(r):
		case i > 0 && (unicode.IsDigit(r) || r == '-' || r == '_'):
		default:
			return false
		}
	}
	return true
}
