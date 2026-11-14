package model

import (
	"errors"
	"math"
	"strings"
)

// ThemeCustomName selects the palette stored in Application.ThemeCustom.
const ThemeCustomName = "custom"

// MinimumPaletteContrast is the lowest accepted contrast ratio between the
// primary and the background color of a custom palette. It is the WCAG 2
// threshold for large text and interface components; below it the settings
// page that would correct the palette is itself hard to read.
const MinimumPaletteContrast = 3.0

var (
	// ErrPaletteColor reports a color that is missing or not a #RRGGBB value.
	ErrPaletteColor = errors.New("a palette color is missing or not a #RRGGBB value")
	// ErrPaletteContrast reports a primary color too close to the background.
	ErrPaletteContrast = errors.New("palette primary color has too little contrast against the background")
)

// Theme color Data Model
type Palette struct {
	Background string `json:"background" yaml:"Background"`
	Primary    string `json:"primary" yaml:"Primary"`
	Accent     string `json:"accent" yaml:"Accent"`
}

// Color modes. A theme can give other token values for light and for dark,
// and the site setting chooses one of the two or leaves the choice to the
// device of the visitor.
const (
	ThemeModeAuto  = "auto"
	ThemeModeLight = "light"
	ThemeModeDark  = "dark"
)

// IsThemeMode reports whether a value is one of the three site settings.
func IsThemeMode(value string) bool {
	return value == ThemeModeAuto || value == ThemeModeLight || value == ThemeModeDark
}

// Theme Data Model
type Theme struct {
	Name   string  `json:"name"`
	Colors Palette `json:"colors"`
	// Tokens holds the style tokens a theme sets besides its three colors,
	// keyed by token name. The values have passed ThemeFile.Validate.
	Tokens map[string]string `json:"tokens,omitempty"`
	// Modes holds the token values a theme replaces in light and in dark
	// mode, keyed by mode and by token name. The three colors can be among
	// them. The values have passed ThemeFile.Validate.
	Modes map[string]map[string]string `json:"-"`
	// Layout holds the layout options a theme specifies, by the names they
	// have under Homepage in config.yml, with values a theme can give.
	Layout map[string]string `json:"-"`
	// Title, Version, Author and License describe a theme read from a file.
	Title   ThemeText `json:"-"`
	Version string    `json:"-"`
	Author  string    `json:"-"`
	License string    `json:"-"`
	// Directory reports a theme that is a directory of the themes directory,
	// read from the theme.yml in it, instead of a single file.
	Directory bool `json:"-"`
}

// DisplayName returns the name to show for the theme in an interface
// language: the name its file gives, or the name of the file.
func (t Theme) DisplayName(locale string) string {
	if title := t.Title.For(locale); title != "" {
		return title
	}
	return t.Name
}

// In returns the theme as it is in one mode: its colors and tokens with the
// values of that mode in place, and no modes of its own. A theme without
// values for the mode is returned with its colors and tokens as they are.
func (t Theme) In(mode string) Theme {
	resolved := t
	resolved.Modes = nil
	replaced := t.Modes[mode]
	if len(replaced) == 0 {
		return resolved
	}
	resolved.Tokens = make(map[string]string, len(t.Tokens)+len(replaced))
	for name, value := range t.Tokens {
		resolved.Tokens[name] = value
	}
	for name, value := range replaced {
		switch name {
		case TokenBackground:
			resolved.Colors.Background = value
		case TokenPrimary:
			resolved.Colors.Primary = value
		case TokenAccent:
			resolved.Colors.Accent = value
		default:
			resolved.Tokens[name] = value
		}
	}
	if len(resolved.Tokens) == 0 {
		resolved.Tokens = nil
	}
	return resolved
}

// UnmarshalYAML decodes the palette without ever failing. config.yml is edited
// by hand, and a ThemeCustom block of the wrong shape must not make the whole
// configuration unreadable. Such a block decodes to an empty palette, which
// Validate rejects.
func (p *Palette) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain Palette
	var value plain
	if err := unmarshal(&value); err != nil {
		value = plain{}
	}
	*p = Palette(value)
	return nil
}

// Normalized trims the colors and writes them in lower case.
func (p Palette) Normalized() Palette {
	clean := func(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
	return Palette{Background: clean(p.Background), Primary: clean(p.Primary), Accent: clean(p.Accent)}
}

// Validate accepts only #RRGGBB colors, because the values are written into
// style attributes and SVG markup, and requires a readable primary color.
func (p Palette) Validate() error {
	for _, value := range []string{p.Background, p.Primary, p.Accent} {
		if !IsHexColor(value) {
			return ErrPaletteColor
		}
	}
	if ContrastRatio(p.Primary, p.Background) < MinimumPaletteContrast {
		return ErrPaletteContrast
	}
	return nil
}

// IsHexColor reports whether value is a #RRGGBB color.
func IsHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, c := range []byte(value[1:]) {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// ContrastRatio returns the WCAG 2 contrast ratio of two #RRGGBB colors, from
// 1 (identical luminance) to 21 (black on white). A value that is not a
// #RRGGBB color yields 0.
func ContrastRatio(first, second string) float64 {
	a, okA := relativeLuminance(first)
	b, okB := relativeLuminance(second)
	if !okA || !okB {
		return 0
	}
	if a < b {
		a, b = b, a
	}
	return (a + 0.05) / (b + 0.05)
}

func relativeLuminance(value string) (float64, bool) {
	if !IsHexColor(value) {
		return 0, false
	}
	channel := func(high, low byte) float64 {
		c := float64(hexDigit(high)<<4|hexDigit(low)) / 255
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(value[1], value[2]) + 0.7152*channel(value[3], value[4]) + 0.0722*channel(value[5], value[6]), true
}

// hexDigit converts a digit that IsHexColor has already accepted.
func hexDigit(c byte) uint8 {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}
