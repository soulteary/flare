package model

import (
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// A theme file can declare options: settings of the theme that the
// administrator chooses values for. A value changes tokens and layout options
// of the theme, and is written on the body of the navigation pages for the
// stylesheet of the theme.
const (
	// MaximumThemeOptions is the number of options a theme can declare.
	MaximumThemeOptions = 8
	// MaximumThemeOptionChoices is the number of values an option can have.
	MaximumThemeOptionChoices = 6
)

// Kinds of options.
const (
	// ThemeOptionBoolean is on or off.
	ThemeOptionBoolean = "boolean"
	// ThemeOptionEnum has one of the values the theme lists.
	ThemeOptionEnum = "enum"
	// ThemeOptionColor is a color the administrator picks for one token.
	ThemeOptionColor = "color"
)

// The values of a boolean option, as they are stored and written on a page.
const (
	ThemeOptionOn  = "true"
	ThemeOptionOff = "false"
)

// ErrThemeOption reports an option of a theme file that is not accepted.
var ErrThemeOption = errors.New("theme file has an option that is not accepted")

// ThemeOption is one option of a theme.
type ThemeOption struct {
	// ID names the option in the theme file, in config.yml and on the page.
	ID   string
	Kind string
	// Label is shown with the option on the settings page. Without it the
	// ID is shown.
	Label ThemeText
	// Default is the value of the option until the administrator chooses
	// another: one of the values of Choices, or a color. A color option
	// that gives none in the file has the color the theme gives its token.
	Default string
	// Choices are the values of an enum option in the order of the file.
	// A boolean option has two: off, which changes nothing, and on.
	Choices []ThemeOptionChoice
	// Token is the token a color option gives its color to.
	Token string
}

// ThemeOptionChoice is a value of an option with what it changes: tokens and
// layout options, written as they are at the top of a theme file.
type ThemeOptionChoice struct {
	Value  string
	Label  ThemeText
	Tokens map[string]string
	Layout map[string]string
}

// ThemeAttribute is the value of an option as a page writes it on its body,
// as data-theme-<name>="<value>". Both are identifiers of the theme file that
// have passed Validate, and safe to write into markup as they are.
type ThemeAttribute struct {
	Name  string
	Value string
}

// themeOptionTokens are the tokens a value of an option can set: all but the
// background and the primary color. The contrast between those two is
// checked when a theme file is read, and stays as it was checked whatever the
// administrator chooses.
var themeOptionTokens = []string{
	TokenAccent, TokenIcon, TokenMuted, TokenBorder, TokenFocus, TokenInverse,
	TokenHover, TokenRadius, TokenFontFamily, TokenFontScale,
}

// themeOptionColors are the tokens a color option can give its color to:
// those of the tokens above that hold one opaque color.
var themeOptionColors = []string{TokenAccent, TokenIcon, TokenMuted, TokenBorder, TokenFocus, TokenInverse}

// DisplayName returns the label of an option for an interface language, or
// the ID of the option where it has none.
func (o ThemeOption) DisplayName(locale string) string {
	if label := o.Label.For(locale); label != "" {
		return label
	}
	return o.ID
}

// DisplayName returns the label of a value for an interface language, or the
// value itself where it has none.
func (c ThemeOptionChoice) DisplayName(locale string) string {
	if label := c.Label.For(locale); label != "" {
		return label
	}
	return c.Value
}

// accepts reports whether an option can have a value.
func (o ThemeOption) accepts(value string) bool {
	if o.Kind == ThemeOptionColor {
		return IsHexColor(value)
	}
	return slices.ContainsFunc(o.Choices, func(choice ThemeOptionChoice) bool { return choice.Value == value })
}

// Value returns the value an option has for what is stored for it: what is
// stored if the option can have that value, and its default otherwise. A
// value that was stored for an earlier version of the theme, or written into
// config.yml by hand, therefore never reaches a page unchecked.
func (o ThemeOption) Value(stored string) string {
	if o.accepts(stored) {
		return stored
	}
	return o.Default
}

// IsDefault reports whether a value is the default of the option. A color is
// the same color in either case: a browser submits one in lower case,
// whatever the case of the theme file.
func (o ThemeOption) IsDefault(value string) bool {
	if o.Kind == ThemeOptionColor {
		return strings.EqualFold(value, o.Default)
	}
	return value == o.Default
}

// tokenColor returns the color a theme gives a token a color option can
// name, in either mode where it has no other for one, or nothing where it
// gives the token none.
func (t Theme) tokenColor(token string) string {
	if token == TokenAccent {
		return t.Colors.Accent
	}
	return t.Tokens[token]
}

// WithOptions returns the theme with the values of its options in place:
// those chosen by the ID of each option, and the default of an option nothing
// usable is chosen for. A token a value sets holds over the token of the
// theme and over the values the theme has for light and for dark; a layout
// option holds over the layout of the theme. A color option that has the
// color the theme gives its token sets nothing, so that the theme keeps what
// it has for light and for dark until another color is picked. The values of
// boolean and enum options are the attributes of the result. A theme without
// options is returned as it is.
func (t Theme) WithOptions(chosen map[string]string) Theme {
	if len(t.Options) == 0 {
		return t
	}
	resolved := t
	resolved.Tokens = maps.Clone(t.Tokens)
	resolved.Layout = maps.Clone(t.Layout)
	resolved.Attributes = nil
	if len(t.Modes) > 0 {
		resolved.Modes = make(map[string]map[string]string, len(t.Modes))
		for mode, replaced := range t.Modes {
			resolved.Modes[mode] = maps.Clone(replaced)
		}
	}
	set := func(token, value string) {
		if token == TokenAccent {
			resolved.Colors.Accent = value
		} else {
			if resolved.Tokens == nil {
				resolved.Tokens = make(map[string]string)
			}
			resolved.Tokens[token] = value
		}
		for _, replaced := range resolved.Modes {
			delete(replaced, token)
		}
	}
	for _, option := range t.Options {
		value := option.Value(chosen[option.ID])
		if option.Kind == ThemeOptionColor {
			if !strings.EqualFold(value, t.tokenColor(option.Token)) {
				set(option.Token, value)
			}
			continue
		}
		resolved.Attributes = append(resolved.Attributes, ThemeAttribute{Name: option.ID, Value: value})
		for _, choice := range option.Choices {
			if choice.Value != value {
				continue
			}
			for _, token := range themeOptionTokens {
				if replaced, given := choice.Tokens[token]; given {
					set(token, replaced)
				}
			}
			for layout, replaced := range choice.Layout {
				if resolved.Layout == nil {
					resolved.Layout = make(map[string]string)
				}
				resolved.Layout[layout] = replaced
			}
		}
	}
	return resolved
}

// readThemeOptions reads the options of a theme file: a list of mappings.
// Parts this release does not know are left out and returned as paths in the
// file, like any other: an option of a kind it does not have, a key of an
// option or of a value, and tokens and layout options of a value. What is
// used is checked by Validate.
func readThemeOptions(node yaml.Node) ([]ThemeOption, ThemeFileUnused, error) {
	var unused ThemeFileUnused
	var written []map[string]yaml.Node
	if err := node.Decode(&written); err != nil {
		return nil, unused, themeFileError(ErrThemeSyntax, "options: "+err.Error())
	}
	options := make([]ThemeOption, 0, len(written))
	for i, parts := range written {
		id, named := themeScalar(parts["id"], "!!str")
		if !named || !isThemeWord(id) {
			return nil, unused, themeFileError(ErrThemeOption, "options["+strconv.Itoa(i+1)+"].id")
		}
		path := "options." + id
		kind, typed := themeScalar(parts["type"], "!!str")
		if !typed {
			return nil, unused, themeFileError(ErrThemeOption, path+".type")
		}
		if kind != ThemeOptionBoolean && kind != ThemeOptionEnum && kind != ThemeOptionColor {
			// A kind of a later release: the theme is used without the option.
			unused.Keys = append(unused.Keys, path)
			continue
		}
		option, err := readThemeOption(ThemeOption{ID: id, Kind: kind}, parts, path, &unused)
		if err != nil {
			return nil, unused, err
		}
		options = append(options, option)
	}
	return options, unused, nil
}

// readThemeOption reads the keys of one option of a known kind.
func readThemeOption(option ThemeOption, parts map[string]yaml.Node, path string, unused *ThemeFileUnused) (ThemeOption, error) {
	// A boolean option says what it changes when it is on beside its other
	// keys; that is its second value.
	on := ThemeOptionChoice{Value: ThemeOptionOn}
	defaulted := false
	for _, key := range sortedKeys(parts) {
		part := parts[key]
		switch {
		case key == "id" || key == "type":
		case key == "label":
			if err := part.Decode(&option.Label); err != nil {
				return option, themeFileError(ErrThemeSyntax, path+".label: "+err.Error())
			}
		case key == "default":
			value, accepted := readThemeOptionDefault(option.Kind, part)
			if !accepted {
				return option, themeFileError(ErrThemeOption, path+".default")
			}
			option.Default, defaulted = value, true
		case key == "choices" && option.Kind == ThemeOptionEnum:
			choices, err := readThemeOptionChoices(part, path, unused)
			if err != nil {
				return option, err
			}
			option.Choices = choices
		case key == "tokens" && option.Kind == ThemeOptionBoolean:
			if err := part.Decode(&on.Tokens); err != nil {
				return option, themeFileError(ErrThemeSyntax, path+".tokens: "+err.Error())
			}
		case key == "layout" && option.Kind == ThemeOptionBoolean:
			if err := part.Decode(&on.Layout); err != nil {
				return option, themeFileError(ErrThemeSyntax, path+".layout: "+err.Error())
			}
		case key == "token" && option.Kind == ThemeOptionColor:
			token, text := themeScalar(part, "!!str")
			if !text {
				return option, themeFileError(ErrThemeOption, path+".token")
			}
			option.Token = token
		default:
			unused.Keys = append(unused.Keys, path+"."+key)
		}
	}
	switch option.Kind {
	case ThemeOptionBoolean:
		dropUnknownParts(&on, path+".", unused)
		option.Choices = []ThemeOptionChoice{{Value: ThemeOptionOff}, on}
		if !defaulted {
			option.Default = ThemeOptionOff
		}
	case ThemeOptionEnum:
		if !defaulted && len(option.Choices) > 0 {
			option.Default = option.Choices[0].Value
		}
	}
	return option, nil
}

// readThemeOptionDefault reads the default of an option as the value it is
// stored as: true or false for a boolean option, and the text of a single
// value for the other kinds.
func readThemeOptionDefault(kind string, node yaml.Node) (string, bool) {
	if kind == ThemeOptionBoolean {
		value, written := themeScalar(node, "!!bool")
		on, err := strconv.ParseBool(value)
		return strconv.FormatBool(on), written && err == nil
	}
	return themeText(node)
}

// themeText returns a single value of a theme file as text, whatever it is
// written as: a value of an option can be a word such as compact or a number
// such as 4. A value that is left empty is none.
func themeText(node yaml.Node) (string, bool) {
	if node.Kind == yaml.AliasNode && node.Alias != nil {
		node = *node.Alias
	}
	if node.Kind != yaml.ScalarNode || node.ShortTag() == "!!null" {
		return "", false
	}
	return node.Value, true
}

// readThemeOptionChoices reads the values of an enum option.
func readThemeOptionChoices(node yaml.Node, path string, unused *ThemeFileUnused) ([]ThemeOptionChoice, error) {
	var written []map[string]yaml.Node
	if err := node.Decode(&written); err != nil {
		return nil, themeFileError(ErrThemeSyntax, path+".choices: "+err.Error())
	}
	choices := make([]ThemeOptionChoice, 0, len(written))
	for i, parts := range written {
		value, named := themeText(parts["value"])
		if !named || !isThemeWord(value) {
			return nil, themeFileError(ErrThemeOption, path+".choices["+strconv.Itoa(i+1)+"].value")
		}
		choice := ThemeOptionChoice{Value: value}
		at := path + ".choices." + value + "."
		for _, key := range sortedKeys(parts) {
			part := parts[key]
			var target any
			switch key {
			case "value":
				continue
			case "label":
				target = &choice.Label
			case "tokens":
				target = &choice.Tokens
			case "layout":
				target = &choice.Layout
			default:
				unused.Keys = append(unused.Keys, at+key)
				continue
			}
			if err := part.Decode(target); err != nil {
				return nil, themeFileError(ErrThemeSyntax, at+key+": "+err.Error())
			}
		}
		dropUnknownParts(&choice, at, unused)
		choices = append(choices, choice)
	}
	return choices, nil
}

// dropUnknownParts removes the tokens and the layout options of a value that
// this release does not know, and a layout value it does not have, and adds
// their paths to what is not used.
func dropUnknownParts(choice *ThemeOptionChoice, path string, unused *ThemeFileUnused) {
	unused.Tokens = dropUnknownTokens(choice.Tokens, path+"tokens.", unused.Tokens)
	unused.Keys = dropUnknownLayout(choice.Layout, path+"layout.", unused.Keys)
}

// sortedKeys returns the keys of a mapping of a theme file in a fixed order.
func sortedKeys(parts map[string]yaml.Node) []string {
	keys := make([]string, 0, len(parts))
	for key := range parts {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// validateThemeOptions accepts options whose values can be offered on the
// settings page, stored in config.yml and written into a page. The first
// reason found is returned as a *ThemeFileError that names the part of the
// file, and the order of the checks does not vary. The options of a file
// that is not a whole theme yet can have a color option without a default:
// it takes the color of a token of the theme the file is based on.
func validateThemeOptions(options []ThemeOption, whole bool) error {
	if len(options) > MaximumThemeOptions {
		return themeFileError(ErrThemeOption, "options")
	}
	seen := make(map[string]bool, len(options))
	for i, option := range options {
		if !isThemeWord(option.ID) {
			return themeFileError(ErrThemeOption, "options["+strconv.Itoa(i+1)+"].id")
		}
		path := "options." + option.ID
		if seen[option.ID] {
			return themeFileError(ErrThemeOption, path)
		}
		seen[option.ID] = true
		if err := validateThemeLabel(option.Label, path+".label"); err != nil {
			return err
		}
		switch option.Kind {
		case ThemeOptionColor:
			if !slices.Contains(themeOptionColors, option.Token) {
				return themeFileError(ErrThemeOption, path+".token")
			}
		case ThemeOptionBoolean:
			if len(option.Choices) != 2 || option.Choices[0].Value != ThemeOptionOff || option.Choices[1].Value != ThemeOptionOn {
				return themeFileError(ErrThemeOption, path)
			}
			if err := validateThemeOptionChoice(option.Choices[1], path+"."); err != nil {
				return err
			}
		case ThemeOptionEnum:
			if len(option.Choices) < 2 || len(option.Choices) > MaximumThemeOptionChoices {
				return themeFileError(ErrThemeOption, path+".choices")
			}
			values := make(map[string]bool, len(option.Choices))
			for n, choice := range option.Choices {
				if !isThemeWord(choice.Value) {
					return themeFileError(ErrThemeOption, path+".choices["+strconv.Itoa(n+1)+"].value")
				}
				at := path + ".choices." + choice.Value
				if values[choice.Value] {
					return themeFileError(ErrThemeOption, at)
				}
				values[choice.Value] = true
				if err := validateThemeLabel(choice.Label, at+".label"); err != nil {
					return err
				}
				if err := validateThemeOptionChoice(choice, at+"."); err != nil {
					return err
				}
			}
		default:
			return themeFileError(ErrThemeOption, path+".type")
		}
		if !whole && option.Kind == ThemeOptionColor && option.Default == "" {
			continue
		}
		if !option.accepts(option.Default) {
			return themeFileError(ErrThemeOption, path+".default")
		}
	}
	return nil
}

// validateThemeOptionChoice checks what a value of an option changes: tokens
// a value can set, with values the tokens accept, and layout options with
// values a theme can give.
func validateThemeOptionChoice(choice ThemeOptionChoice, path string) error {
	names := make([]string, 0, len(choice.Tokens))
	for name := range choice.Tokens {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if !slices.Contains(themeOptionTokens, name) {
			return themeFileError(ErrThemeOption, path+"tokens."+name)
		}
	}
	for _, name := range themeOptionTokens {
		if value, set := choice.Tokens[name]; set && !themeTokenChecks[name](value) {
			return themeFileError(ErrThemeTokenValue, path+"tokens."+name)
		}
	}
	layout := make([]string, 0, len(choice.Layout))
	for option := range choice.Layout {
		layout = append(layout, option)
	}
	slices.Sort(layout)
	for _, option := range layout {
		if !IsThemeLayoutValue(option, choice.Layout[option]) {
			return themeFileError(ErrThemeSyntax, path+"layout."+option)
		}
	}
	return nil
}

// validateThemeLabel accepts a label that is plain text of the length of a
// name, in every language it is given in.
func validateThemeLabel(label ThemeText, path string) error {
	if !isThemeText(label.Plain, maximumThemeNameLength) {
		return themeFileError(ErrThemeText, path)
	}
	for _, language := range label.languages() {
		if !isThemeWord(language) || !isThemeText(label.Localized[language], maximumThemeNameLength) {
			return themeFileError(ErrThemeText, path+": "+strconv.Quote(language))
		}
	}
	return nil
}
