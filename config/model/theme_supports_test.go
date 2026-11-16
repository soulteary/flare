package model

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// A theme file can list the layout options that still work on pages written
// with its templates. A file that does not say is not one that names none.
func TestThemeFileSupports(t *testing.T) {
	for name, tc := range map[string]struct {
		written string
		want    []string
	}{
		"not written":         {"", nil},
		"without a value":     {"supports:\n", nil},
		"written as null":     {"supports: ~\n", nil},
		"an empty list":       {"supports: []\n", []string{}},
		"one option":          {"supports:\n  - DesktopColumns\n", []string{LayoutDesktopColumns}},
		"in a line":           {"supports: [CategoryNavigation, TitleSize]\n", []string{LayoutTitleSize, LayoutCategoryNavigation}},
		"an option twice":     {"supports: [TitleSize, TitleSize]\n", []string{LayoutTitleSize}},
		"every option":        {"supports: [DesktopColumns, BookmarkLayout, CategoryNavigation, HideAppDescriptions, TitleSize, MobileBookmarkLayout]\n", ThemeLayoutOptions},
		"an option and a key": {"supports:\n  - BookmarkLayout\nlayout:\n  TitleSize: compact\n", []string{LayoutBookmarkLayout}},
	} {
		file, unused, err := ReadThemeFile([]byte(validThemeFile + tc.written))
		if err != nil || len(unused.Keys) != 0 {
			t.Fatalf("%s: %v %+v", name, err, unused)
		}
		if (file.Supports == nil) != (tc.want == nil) || !slices.Equal(file.Supports, tc.want) {
			t.Errorf("%s: got %#v, want %#v", name, file.Supports, tc.want)
		}
		theme := file.Theme("zelda")
		if (theme.Supports == nil) != (tc.want == nil) || !slices.Equal(theme.Supports, tc.want) {
			t.Errorf("%s: the theme has %#v, want %#v", name, theme.Supports, tc.want)
		}
		if len(file.Supports) > 0 {
			// The theme owns its list.
			file.Supports[0] = "changed"
			if theme.Supports[0] == "changed" {
				t.Errorf("%s: the theme shares its list with the file it was read from", name)
			}
		}
	}
}

// A name this release does not have is left out and reported, once; the
// theme and the rest of the list stay in use. What is no list is refused.
func TestThemeFileSupportsFromAnotherRelease(t *testing.T) {
	long := strings.Repeat("值", 40)
	file, unused, err := ReadThemeFile([]byte(validThemeFile + "supports: [CardStyle, TitleSize, CardStyle, " + long + ", layout]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(file.Supports, []string{LayoutTitleSize}) {
		t.Fatalf("in use: %#v", file.Supports)
	}
	if want := []string{"supports.CardStyle", "supports.layout", "supports." + strings.Repeat("值", 32) + "…"}; !slices.Equal(unused.Keys, want) {
		t.Fatalf("unused: got %q, want %q", unused.Keys, want)
	}
	// Only unknown names leave an empty list, which names none.
	file, _, err = ReadThemeFile([]byte(validThemeFile + "supports: [CardStyle]\n"))
	if err != nil || file.Supports == nil || len(file.Supports) != 0 {
		t.Fatalf("%v %#v", err, file.Supports)
	}

	for name, written := range map[string]string{
		"a text":              "supports: DesktopColumns\n",
		"a mapping":           "supports:\n  DesktopColumns: true\n",
		"a list of lists":     "supports:\n  - [DesktopColumns]\n",
		"a list of a mapping": "supports:\n  - layout: DesktopColumns\n",
	} {
		_, _, err = ReadThemeFile([]byte(validThemeFile + written))
		var refused *ThemeFileError
		if !errors.Is(err, ErrThemeSyntax) || !errors.As(err, &refused) || !strings.HasPrefix(refused.Subject, "supports: ") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A theme that is based on another says what its templates support, or
// else has what the theme below says.
func TestThemeFileSupportsOnAnotherTheme(t *testing.T) {
	read := func(written string) ThemeFile {
		t.Helper()
		file, _, err := ReadThemeFile([]byte(written))
		if err != nil {
			t.Fatal(err)
		}
		return file
	}
	for name, tc := range map[string]struct {
		below, above string
		want         []string
	}{
		"neither says":             {"", "", nil},
		"the theme below says":     {"supports: [TitleSize]\n", "", []string{LayoutTitleSize}},
		"the theme below has none": {"supports: []\n", "", []string{}},
		"the theme says":           {"supports: [TitleSize]\n", "supports: [DesktopColumns]\n", []string{LayoutDesktopColumns}},
		"the theme has none":       {"supports: [TitleSize]\n", "supports: []\n", []string{}},
		"only the theme says":      {"", "supports: [DesktopColumns]\n", []string{LayoutDesktopColumns}},
	} {
		base := read(validThemeFile + tc.below)
		whole, err := read("contract: 1\nextends: zelda\n" + tc.above).On(base)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if (whole.Supports == nil) != (tc.want == nil) || !slices.Equal(whole.Supports, tc.want) {
			t.Errorf("%s: got %#v, want %#v", name, whole.Supports, tc.want)
		}
		if len(base.Supports) > 0 && len(whole.Supports) > 0 {
			whole.Supports[0] = "changed"
			if base.Supports[0] == "changed" {
				t.Errorf("%s: the theme shares its list with the theme below", name)
			}
		}
	}
}

// The options a theme with templates does not support are those it neither
// names nor specifies itself.
func TestThemeLayoutUnsupported(t *testing.T) {
	if got := ThemeLayoutUnsupported(nil, nil); got != nil {
		t.Fatalf("a theme that does not say: %v", got)
	}
	if got := ThemeLayoutUnsupported([]string{}, nil); !slices.Equal(got, ThemeLayoutOptions) {
		t.Fatalf("a theme that names none: %v", got)
	}
	if got := ThemeLayoutUnsupported(ThemeLayoutOptions, nil); got != nil {
		t.Fatalf("a theme that names all: %v", got)
	}
	got := ThemeLayoutUnsupported([]string{LayoutDesktopColumns, LayoutTitleSize}, map[string]string{LayoutBookmarkLayout: "sections"})
	if want := []string{LayoutMobileBookmarkLayout, LayoutHideAppDescriptions, LayoutCategoryNavigation}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
