package model

import (
	"cmp"
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
	TokenFontScale  = "font-scale"
)

// ThemeTokenNames lists the tokens in the order of their documentation. The
// first three are the colors every theme has to give.
var ThemeTokenNames = []string{
	TokenBackground, TokenPrimary, TokenAccent,
	TokenIcon, TokenMuted, TokenBorder, TokenFocus, TokenInverse,
	TokenHover, TokenRadius, TokenFontFamily, TokenFontScale,
}

// MaximumThemeRadius is the largest value of the radius token, in pixels.
// Stylesheets use up to three times the token for larger corners.
const MaximumThemeRadius = 32

// maximumFontFamilyLength bounds the font-family token.
const maximumFontFamilyLength = 160

// The font-scale token is a number between these two, in hundredths. Below
// the first, text at the default size falls under 11 pixels; above the
// second, the columns of the page no longer hold their text.
const (
	minimumThemeFontScale = 80
	maximumThemeFontScale = 150
)

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
	// ErrThemeRequires reports a theme file that needs a later release.
	ErrThemeRequires = errors.New("theme file needs a later release")
	// ErrThemeRequiresValue reports a requires that does not name a release.
	ErrThemeRequiresValue = errors.New("theme file names the release it needs in a form that is not accepted")
	// ErrThemeBackground reports a background that is not written in the
	// accepted form.
	ErrThemeBackground = errors.New("theme file has a background that is not accepted")
	// ErrThemeBackgroundAsset reports a background image that is not among
	// the images in the assets of the theme. A theme that is a single file
	// has no assets.
	ErrThemeBackgroundAsset = errors.New("theme file names a background image that is not among the assets of the theme")
	// ErrThemeExtends reports an extends that is not the name of a theme.
	ErrThemeExtends = errors.New("theme file names the theme it is based on in a form that is not accepted")
	// ErrThemeBase reports a theme file based on a theme that is not in use.
	ErrThemeBase = errors.New("theme file is based on a theme that is not in use")
	// ErrThemeBaseCycle reports a theme file that is based on itself, by way
	// of the themes it is based on.
	ErrThemeBaseCycle = errors.New("theme file is based on itself")
	// ErrThemeBaseDepth reports a theme file with more themes below it, one
	// based on another, than a theme can have.
	ErrThemeBaseDepth = errors.New("theme file is based on more themes, one on another, than a theme can be")
)

// MaximumThemeBases is the number of themes a theme can have below it: the
// theme it is based on, the theme that one is based on, and so on.
const MaximumThemeBases = 3

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
	// Layout holds the layout options the theme specifies, by the names the
	// options have under Homepage in config.yml.
	Layout map[string]string
	// Supports lists the layout options that still work on pages written
	// with the templates of the theme, in the order of the appearance
	// page. It is nil for a file that does not say, and empty for one that
	// names none.
	Supports []string
	// Requires names the first release the theme works with, as three
	// numbers such as 0.7.0. It is empty for a theme that names none.
	Requires string
	// Extends names the theme this one is based on. A file that names one
	// gives only what it has of its own, and is a theme once it is read On
	// that theme. It is empty for a theme that stands on its own.
	Extends string
	// Background is the image the theme lays behind the navigation pages.
	// It is nil for a theme without one.
	Background *ThemeBackground
	// Options are the settings the theme offers to the administrator, in
	// the order of the file.
	Options []ThemeOption
}

// ThemeBackground is the background image of a theme. Image names an image
// among the assets of the theme the way its stylesheet does, as
// assets/<name>. Dim is how much of the background color of the theme lies
// over the image, in percent: 0 shows the image as it is, and with 100
// nothing of it is seen. Text is drawn on the image, so Dim is what keeps it
// readable on an image that was not made for text.
type ThemeBackground struct {
	Image string
	Dim   int
}

// Asset returns the name of the image among the assets of the theme. It
// reports false for an image that is not written as assets/<name> with the
// name of an image a theme can bring.
func (b ThemeBackground) Asset() (string, bool) {
	name, named := ThemeAssetReference(b.Image)
	if kind, _ := ThemeAssetType(name); !named || !strings.HasPrefix(kind, "image/") {
		return "", false
	}
	return name, true
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
// used, as paths in the file. Keys holds keys, modes, layout options and
// layout values this release does not know, such as options, modes.sepia,
// layout.CardStyle or "layout.DesktopColumns: 12"; Tokens holds tokens it
// does not know, such as tokens.shadow or modes.light.shadow. A file written
// for a later release remains usable without them.
type ThemeFileUnused struct {
	Keys   []string
	Tokens []string
}

// Empty reports whether every part of the file was used.
func (u ThemeFileUnused) Empty() bool { return len(u.Keys) == 0 && len(u.Tokens) == 0 }

// ReadThemeFile reads and validates a theme file. Keys, modes and tokens this
// release does not know are left out and reported; everything that is used
// has to be valid, or the file is rejected as a whole. A rejection is a
// *ThemeFileError. The release a file needs is checked for its form only:
// ReadThemeFileFor compares it with a release.
//
// A file that names a theme it is based on, in Extends, is read as far as it
// goes: each of its parts is checked, and what only holds for a whole theme,
// such as the three colors and their contrast, is checked by On, with the
// theme it is based on.
func ReadThemeFile(raw []byte) (ThemeFile, ThemeFileUnused, error) {
	return ReadThemeFileFor(raw, "")
}

// ReadThemeFileFor is ReadThemeFile for an application of a given version. A
// file that needs a later release is rejected. A version that is not that of
// a release, as a development build has, meets every requirement.
func ReadThemeFileFor(raw []byte, version string) (ThemeFile, ThemeFileUnused, error) {
	// A mapping of nodes rejects a document that is not a mapping and one
	// that repeats a key.
	var document map[string]yaml.Node
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return ThemeFile{}, ThemeFileUnused{}, themeFileError(ErrThemeSyntax, err.Error())
	}
	if document == nil {
		return ThemeFile{}, ThemeFileUnused{}, themeFileError(ErrThemeSyntax, "the file is empty")
	}
	var file ThemeFile
	// What a file says about itself is read before the rest. A file for
	// another contract or for a later release can hold parts this release
	// cannot read or does not accept, and the reason to give for such a file
	// is the one it states.
	for _, key := range []string{"contract", "requires"} {
		node, written := document[key]
		if !written {
			continue
		}
		var target any = &file.Contract
		if key == "requires" {
			target = &file.Requires
		}
		if err := node.Decode(target); err != nil {
			return ThemeFile{}, ThemeFileUnused{}, themeFileError(ErrThemeSyntax, key+": "+err.Error())
		}
	}
	if file.Contract != ThemeContract {
		return ThemeFile{}, ThemeFileUnused{}, themeFileError(ErrThemeContract, strconv.Itoa(file.Contract))
	}
	if _, written := document["requires"]; written {
		needed, accepted := themeRelease(file.Requires, false)
		if !accepted {
			return ThemeFile{}, ThemeFileUnused{}, themeFileError(ErrThemeRequiresValue, shortThemeValue(file.Requires))
		}
		if running, known := themeRelease(version, true); known && slices.Compare(needed[:], running[:]) > 0 {
			return ThemeFile{}, ThemeFileUnused{}, themeFileError(ErrThemeRequires, file.Requires)
		}
	}
	keys := make([]string, 0, len(document))
	for key := range document {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var unused ThemeFileUnused
	for _, key := range keys {
		var target any
		switch key {
		case "contract", "requires":
			continue
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
		case "layout":
			target = &file.Layout
		case "supports":
			supported, parts, err := readThemeSupports(document[key])
			if err != nil {
				return ThemeFile{}, ThemeFileUnused{}, err
			}
			file.Supports = supported
			unused.Keys = append(unused.Keys, parts...)
			continue
		case "extends":
			// A name can be written as a number, as 2048 is.
			base, written := themeText(document[key])
			if !written {
				return ThemeFile{}, ThemeFileUnused{}, themeFileError(ErrThemeExtends, "extends")
			}
			if !isThemeWord(base) {
				return ThemeFile{}, ThemeFileUnused{}, themeFileError(ErrThemeExtends, shortThemeValue(base))
			}
			file.Extends = base
			continue
		case "background":
			background, parts, err := readThemeBackground(document[key])
			if err != nil {
				return ThemeFile{}, ThemeFileUnused{}, err
			}
			file.Background = background
			unused.Keys = append(unused.Keys, parts...)
			continue
		case "options":
			options, parts, err := readThemeOptions(document[key])
			if err != nil {
				return ThemeFile{}, ThemeFileUnused{}, err
			}
			file.Options = options
			unused.Keys = append(unused.Keys, parts.Keys...)
			unused.Tokens = append(unused.Tokens, parts.Tokens...)
			continue
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
	unused.Keys = dropUnknownLayout(file.Layout, "layout.", unused.Keys)
	slices.Sort(unused.Keys)
	slices.Sort(unused.Tokens)
	// A file that is based on another theme is checked as a whole with
	// that theme, by On.
	if err := file.validate(file.Extends == ""); err != nil {
		return ThemeFile{}, unused, err
	}
	return file, unused, nil
}

// readThemeSupports reads the layout options a theme file says its
// templates support: a list of the names the options have under layout. A
// name this release does not know is left out and returned as a path in the
// file; a name that is written twice counts once. A list without a value
// says nothing, and an empty list names none.
func readThemeSupports(node yaml.Node) ([]string, []string, error) {
	if node.Kind == yaml.ScalarNode && node.Tag == "!!null" {
		return nil, nil, nil
	}
	var written []string
	if err := node.Decode(&written); err != nil {
		return nil, nil, themeFileError(ErrThemeSyntax, "supports: "+err.Error())
	}
	var unused []string
	for _, option := range written {
		if !IsThemeLayoutOption(option) && !slices.Contains(unused, "supports."+shortThemeValue(option)) {
			unused = append(unused, "supports."+shortThemeValue(option))
		}
	}
	supported := []string{}
	for _, option := range ThemeLayoutOptions {
		if slices.Contains(written, option) {
			supported = append(supported, option)
		}
	}
	return supported, unused, nil
}

// readThemeBackground reads the background of a theme file: a mapping with
// an image and, optionally, how much the image is dimmed. A part this release
// does not know is left out and returned as a path in the file, like any
// other. A background without a value is no background.
//
// The two values are read as what they are written as. A decoder would turn
// a number with a fraction into a whole one, and dim: 0.6 into no dimming at
// all, so anything but a whole number is refused instead.
func readThemeBackground(node yaml.Node) (*ThemeBackground, []string, error) {
	var written map[string]yaml.Node
	if err := node.Decode(&written); err != nil {
		return nil, nil, themeFileError(ErrThemeSyntax, "background: "+err.Error())
	}
	if written == nil {
		return nil, nil, nil
	}
	parts := make([]string, 0, len(written))
	for part := range written {
		parts = append(parts, part)
	}
	slices.Sort(parts)
	background := &ThemeBackground{}
	var unused []string
	for _, part := range parts {
		switch part {
		case "image":
			image, text := themeScalar(written[part], "!!str")
			if !text {
				return nil, nil, themeFileError(ErrThemeBackground, "background.image")
			}
			background.Image = image
		case "dim":
			number, whole := themeScalar(written[part], "!!int")
			dim, err := strconv.Atoi(number)
			if !whole || err != nil {
				return nil, nil, themeFileError(ErrThemeBackground, "background.dim")
			}
			background.Dim = dim
		default:
			unused = append(unused, "background."+part)
		}
	}
	return background, unused, nil
}

// themeScalar returns a value of a theme file as it is written, if it is a
// single value of the given kind: !!str for a text, !!int for a whole number.
func themeScalar(node yaml.Node, kind string) (string, bool) {
	if node.Kind == yaml.AliasNode && node.Alias != nil {
		node = *node.Alias
	}
	if node.Kind != yaml.ScalarNode || node.ShortTag() != kind {
		return "", false
	}
	return node.Value, true
}

// themeRelease reads a release as three numbers: 0.7.0. A theme file names a
// release in exactly that form. The version of an application can also start
// with a v and end with a suffix, as v0.7.0, 0.7.0-rc.1 and 0.7.0+build do;
// such a version counts as the release its numbers name. Anything else is no
// release: the second result is false.
func themeRelease(text string, version bool) ([3]int, bool) {
	var release [3]int
	if version {
		text = strings.TrimPrefix(text, "v")
		if suffix := strings.IndexAny(text, "-+"); suffix >= 0 {
			text = text[:suffix]
		}
	}
	parts := strings.Split(text, ".")
	if len(parts) != len(release) {
		return release, false
	}
	for i, part := range parts {
		// A number is written with digits alone and without leading zeros,
		// and is short enough to hold.
		if part == "" || len(part) > 6 || (len(part) > 1 && part[0] == '0') {
			return release, false
		}
		for _, digit := range []byte(part) {
			if digit < '0' || digit > '9' {
				return release, false
			}
			release[i] = release[i]*10 + int(digit-'0')
		}
	}
	return release, true
}

// shortThemeValue bounds a value of a theme file that is reported back.
func shortThemeValue(value string) string {
	const limit = 32
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	return string([]rune(value)[:limit]) + "…"
}

// dropUnknownLayout removes the layout options this release does not have
// from one mapping of a theme file, and the options with a value it does not
// have, and adds their paths to the list. Such an option is left out like any
// other part a release does not know: the site setting applies instead.
func dropUnknownLayout(layout map[string]string, path string, unused []string) []string {
	for option, value := range layout {
		switch {
		case !IsThemeLayoutOption(option):
			unused = append(unused, path+option)
		case !IsThemeLayoutValue(option, value):
			unused = append(unused, path+option+": "+shortThemeValue(value))
		default:
			continue
		}
		delete(layout, option)
	}
	return unused
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
	// A file that is used in full stands on its own.
	if file.Extends != "" {
		return ThemeFile{}, themeFileError(ErrThemeBase, file.Extends)
	}
	return file, nil
}

// Validate accepts a theme whose tokens can be written into a page. The three
// colors are required and have to pass the checks of a custom palette; the
// other tokens are optional. The values of a mode replace those of the theme,
// and the result has to pass the same checks. The first reason found is
// returned as a *ThemeFileError that names the part of the file, and the
// order of the checks does not vary.
func (f ThemeFile) Validate() error { return f.validate(true) }

// validate is Validate for a file that is a whole theme, and the checks of
// its parts alone for a file that is based on another theme and is not read
// On it yet. Such a file need not give the three colors, their contrast
// depends on what the other theme gives, and a color option can take its
// default from a token of that theme.
func (f ThemeFile) validate(whole bool) error {
	if f.Contract != ThemeContract {
		return themeFileError(ErrThemeContract, strconv.Itoa(f.Contract))
	}
	if _, accepted := themeRelease(f.Requires, false); f.Requires != "" && !accepted {
		return themeFileError(ErrThemeRequiresValue, shortThemeValue(f.Requires))
	}
	if f.Extends != "" && !isThemeWord(f.Extends) {
		return themeFileError(ErrThemeExtends, shortThemeValue(f.Extends))
	}
	if err := validateThemeTokens(f.Tokens, "tokens."); err != nil {
		return err
	}
	if whole {
		for _, name := range ThemeTokenNames[:3] {
			if _, set := f.Tokens[name]; !set {
				return themeFileError(ErrPaletteColor, "tokens."+name)
			}
		}
		if err := f.Palette().Validate(); err != nil {
			return themeFileError(err, "")
		}
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
		if !whole {
			continue
		}
		if err := f.Theme("").In(mode).Colors.Validate(); err != nil {
			return themeFileError(err, "modes."+mode)
		}
	}
	options := make([]string, 0, len(f.Layout))
	for option := range f.Layout {
		options = append(options, option)
	}
	slices.Sort(options)
	for _, option := range options {
		if !IsThemeLayoutValue(option, f.Layout[option]) {
			return themeFileError(ErrThemeSyntax, "layout."+option)
		}
	}
	if err := validateThemeOptions(f.optionsWithDefaults(), whole); err != nil {
		return err
	}
	if f.Background != nil {
		if _, named := f.Background.Asset(); !named {
			return themeFileError(ErrThemeBackground, "background.image")
		}
		if f.Background.Dim < 0 || f.Background.Dim > 100 {
			return themeFileError(ErrThemeBackground, "background.dim")
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
	if len(f.Layout) > 0 {
		theme.Layout = maps.Clone(f.Layout)
	}
	if f.Background != nil {
		// Validate has accepted the image, so it names an asset.
		theme.BackgroundImage, _ = f.Background.Asset()
		theme.BackgroundDim = f.Background.Dim
	}
	theme.Options = f.optionsWithDefaults()
	theme.Extends = f.Extends
	if f.Supports != nil {
		theme.Supports = append([]string{}, f.Supports...)
	}
	return theme
}

// optionsWithDefaults returns the options of the file, each with a default.
// A color option that names none has the color the file gives its token, so
// that the color is written in one place. The file keeps such an option as
// it is written: read On another theme, it takes the color that theme and
// the file give the token together.
func (f ThemeFile) optionsWithDefaults() []ThemeOption {
	options := slices.Clone(f.Options)
	for i, option := range options {
		if option.Kind == ThemeOptionColor && option.Default == "" {
			options[i].Default = f.Tokens[option.Token]
		}
	}
	return options
}

// On returns the theme file as it is read on the theme it is based on: with
// what that theme gives where the file gives nothing of its own. The file is
// read as if the parts of the other theme were written first. A token of
// the file replaces the token of the other theme in tokens and in each mode
// of modes, and a layout option replaces the layout option. An option of the
// file replaces the option of the same ID where that one stands, and the
// other options of the file follow those of the other theme. The background
// of the other theme stays unless the file has one. What describes a file,
// its name, version, author, license and the release it needs, is that of
// the file alone.
//
// The result is a whole theme and is checked as one, so what holds only for
// the two together is found here: the contrast of a color of the file with
// a color of the other theme, the number of options, and a color option
// whose token neither gives a color. base is a whole theme itself.
func (f ThemeFile) On(base ThemeFile) (ThemeFile, error) {
	merged := f
	merged.Tokens = overlayThemeValues(base.Tokens, f.Tokens)
	merged.Layout = overlayThemeValues(base.Layout, f.Layout)
	merged.Modes = nil
	for _, mode := range []string{ThemeModeLight, ThemeModeDark} {
		if values := overlayThemeValues(base.Modes[mode], f.Modes[mode]); values != nil {
			if merged.Modes == nil {
				merged.Modes = make(map[string]map[string]string)
			}
			merged.Modes[mode] = values
		}
	}
	// What the templates support is said by the file, or else by the other
	// theme, whose templates the theme has where it brings none.
	if f.Supports == nil && base.Supports != nil {
		merged.Supports = append([]string{}, base.Supports...)
	}
	merged.Options = slices.Clone(base.Options)
	for _, option := range f.Options {
		place := slices.IndexFunc(merged.Options, func(other ThemeOption) bool { return other.ID == option.ID })
		if place < 0 {
			merged.Options = append(merged.Options, option)
			continue
		}
		merged.Options[place] = option
	}
	// The background is that of the file, or else that of the other theme,
	// and belongs to the result alone.
	if source := cmp.Or(f.Background, base.Background); source != nil {
		background := *source
		merged.Background = &background
	}
	if err := merged.Validate(); err != nil {
		return ThemeFile{}, err
	}
	return merged, nil
}

// overlayThemeValues returns the values of one mapping of a theme file with
// those of another in place: what the second gives replaces what the first
// gives. Neither is changed, and two mappings without values give none.
func overlayThemeValues(below, above map[string]string) map[string]string {
	if len(below) == 0 && len(above) == 0 {
		return nil
	}
	values := make(map[string]string, len(below)+len(above))
	maps.Copy(values, below)
	maps.Copy(values, above)
	return values
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
	TokenFontScale:  isThemeFontScale,
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

// isThemeFontScale accepts a number from 0.8 to 1.5 with at most two
// decimals, such as 1, 0.9 or 1.25. Stylesheets multiply lengths by it.
func isThemeFontScale(value string) bool {
	whole, fraction, decimal := strings.Cut(value, ".")
	if (whole != "0" && whole != "1") || (decimal && (fraction == "" || len(fraction) > 2)) {
		return false
	}
	hundredths := int(whole[0]-'0') * 100
	for i := 0; i < len(fraction); i++ {
		if fraction[i] < '0' || fraction[i] > '9' {
			return false
		}
		digit := int(fraction[i] - '0')
		if i == 0 {
			digit *= 10
		}
		hundredths += digit
	}
	return hundredths >= minimumThemeFontScale && hundredths <= maximumThemeFontScale
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
