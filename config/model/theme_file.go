package model

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

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
)

// ThemeFile is a theme as written in a file. Its values end up in a style
// attribute without further escaping, so nothing is used before Validate has
// accepted it.
type ThemeFile struct {
	Contract int               `yaml:"contract"`
	Tokens   map[string]string `yaml:"tokens"`
}

// ParseThemeFile reads and validates one theme file. Unknown keys are errors:
// a key this release does not read would otherwise be ignored silently.
func ParseThemeFile(raw []byte) (ThemeFile, error) {
	var file ThemeFile
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return ThemeFile{}, fmt.Errorf("%w: %v", ErrThemeSyntax, err)
	}
	if err := file.Validate(); err != nil {
		return ThemeFile{}, err
	}
	return file, nil
}

// Validate accepts a theme whose tokens can be written into a page. The three
// colors are required and have to pass the checks of a custom palette; the
// other tokens are optional.
func (f ThemeFile) Validate() error {
	if f.Contract != ThemeContract {
		return fmt.Errorf("%w: contract %d, this release reads %d", ErrThemeContract, f.Contract, ThemeContract)
	}
	for name, value := range f.Tokens {
		check, known := themeTokenChecks[name]
		if !known {
			return fmt.Errorf("%w: %q", ErrThemeToken, name)
		}
		if !check(value) {
			return fmt.Errorf("%w: %q", ErrThemeTokenValue, name)
		}
	}
	if err := f.Palette().Validate(); err != nil {
		return err
	}
	return nil
}

// Palette returns the three colors of the theme as written.
func (f ThemeFile) Palette() Palette {
	return Palette{Background: f.Tokens[TokenBackground], Primary: f.Tokens[TokenPrimary], Accent: f.Tokens[TokenAccent]}
}

// Theme converts a validated file into the theme the application uses.
func (f ThemeFile) Theme(name string) Theme {
	theme := Theme{Name: name, Colors: f.Palette()}
	for _, token := range ThemeTokenNames[3:] {
		if value, set := f.Tokens[token]; set {
			if theme.Tokens == nil {
				theme.Tokens = make(map[string]string)
			}
			theme.Tokens[token] = value
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
