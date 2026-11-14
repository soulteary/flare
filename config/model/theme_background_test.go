package model

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// A theme file can name an image of the assets as the background of the
// navigation pages, and say how much the image is dimmed.
func TestThemeFileBackground(t *testing.T) {
	for name, tc := range map[string]struct {
		written string
		want    *ThemeBackground
		asset   string
	}{
		"none":                {"", nil, ""},
		"without a value":     {"background:\n", nil, ""},
		"a null":              {"background: ~\n", nil, ""},
		"an image":            {"background:\n  image: assets/wallpaper.jpg\n", &ThemeBackground{Image: "assets/wallpaper.jpg"}, "wallpaper.jpg"},
		"an image and a dim":  {"background:\n  image: assets/wallpaper.jpg\n  dim: 60\n", &ThemeBackground{Image: "assets/wallpaper.jpg", Dim: 60}, "wallpaper.jpg"},
		"in one line":         {"background: {image: assets/a.png, dim: 5}\n", &ThemeBackground{Image: "assets/a.png", Dim: 5}, "a.png"},
		"in quotes":           {"background:\n  image: \"assets/a.png\"\n", &ThemeBackground{Image: "assets/a.png"}, "a.png"},
		"not dimmed":          {"background:\n  image: assets/a.png\n  dim: 0\n", &ThemeBackground{Image: "assets/a.png"}, "a.png"},
		"dimmed all the way":  {"background:\n  image: assets/a.png\n  dim: 100\n", &ThemeBackground{Image: "assets/a.png", Dim: 100}, "a.png"},
		"an upper-case name":  {"background:\n  image: assets/Wall-2_x.final.WEBP\n", &ThemeBackground{Image: "assets/Wall-2_x.final.WEBP"}, "Wall-2_x.final.WEBP"},
		"every kind of image": {"background:\n  image: assets/a.avif\n", &ThemeBackground{Image: "assets/a.avif"}, "a.avif"},
		"a dim with a sign":   {"background:\n  image: assets/a.png\n  dim: +7\n", &ThemeBackground{Image: "assets/a.png", Dim: 7}, "a.png"},
	} {
		file, unused, err := ReadThemeFile([]byte(validThemeFile + tc.written))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !unused.Empty() {
			t.Errorf("%s: unused %+v", name, unused)
		}
		if (file.Background == nil) != (tc.want == nil) || (tc.want != nil && *file.Background != *tc.want) {
			t.Errorf("%s: background %+v, want %+v", name, file.Background, tc.want)
		}
		theme := file.Theme("zelda")
		dim := 0
		if tc.want != nil {
			dim = tc.want.Dim
		}
		if theme.BackgroundImage != tc.asset || theme.BackgroundDim != dim {
			t.Errorf("%s: the theme has %q dimmed by %d", name, theme.BackgroundImage, theme.BackgroundDim)
		}
		// A file that is used in full, as a bundled theme is, reads the same.
		if _, err = ParseThemeFile([]byte(validThemeFile + tc.written)); err != nil {
			t.Errorf("%s, in full: %v", name, err)
		}
	}
	for _, kind := range []string{"png", "jpg", "jpeg", "gif", "webp", "avif", "PNG", "JpEg"} {
		if _, _, err := ReadThemeFile([]byte(validThemeFile + "background:\n  image: assets/a." + kind + "\n")); err != nil {
			t.Errorf("an image named a.%s: %v", kind, err)
		}
	}
}

// A part of the background this release does not know is left out and
// reported, like any other part of a theme file.
func TestThemeFileBackgroundReportsWhatItDoesNotUse(t *testing.T) {
	file, unused, err := ReadThemeFile([]byte(validThemeFile + "background:\n  size: contain\n  image: assets/a.png\n  position: top\n  dim: 20\nfilters:\n  blur: 2px\n"))
	if err != nil {
		t.Fatal(err)
	}
	if file.Background == nil || *file.Background != (ThemeBackground{Image: "assets/a.png", Dim: 20}) {
		t.Fatalf("background %+v", file.Background)
	}
	if !slices.Equal(unused.Keys, []string{"background.position", "background.size", "filters"}) || len(unused.Tokens) != 0 {
		t.Fatalf("unused %+v", unused)
	}
	// A file that has to be used in full is not accepted with such a part.
	if _, err = ParseThemeFile([]byte(validThemeFile + "background:\n  image: assets/a.png\n  size: contain\n")); !errors.Is(err, ErrThemeSyntax) {
		t.Fatalf("in full: %v", err)
	}
}

// A value that is written once and referred to, as YAML allows, is read as
// the value it stands for.
func TestThemeFileBackgroundThroughReferences(t *testing.T) {
	shared := "shared:\n  image: &image assets/a.png\n  dim: &dim 30\n  list: &list [assets/a.png]\n"
	file, unused, err := ReadThemeFile([]byte(validThemeFile + shared + "background:\n  image: *image\n  dim: *dim\n"))
	if err != nil {
		t.Fatal(err)
	}
	if file.Background == nil || *file.Background != (ThemeBackground{Image: "assets/a.png", Dim: 30}) {
		t.Fatalf("background %+v", file.Background)
	}
	if !slices.Equal(unused.Keys, []string{"shared"}) {
		t.Fatalf("unused %+v", unused)
	}
	for part, written := range map[string]string{
		"background.image": "background:\n  image: *list\n",
		"background.dim":   "background:\n  image: *image\n  dim: *image\n",
	} {
		_, _, err = ReadThemeFile([]byte(validThemeFile + shared + written))
		var rejection *ThemeFileError
		if !errors.As(err, &rejection) || !errors.Is(err, ErrThemeBackground) || rejection.Subject != part {
			t.Errorf("%s: %v", part, err)
		}
	}
}

func TestThemeFileBackgroundNotAccepted(t *testing.T) {
	for name, tc := range map[string]struct {
		written string
		reason  error
		subject string
	}{
		// The image is written as the stylesheet of a theme refers to it:
		// assets/ and the name of an image among the assets.
		"no image":                {"background:\n  dim: 60\n", ErrThemeBackground, "background.image"},
		"nothing":                 {"background: {}\n", ErrThemeBackground, "background.image"},
		"an empty image":          {"background:\n  image: \"\"\n", ErrThemeBackground, "background.image"},
		"not in assets":           {"background:\n  image: wallpaper.jpg\n", ErrThemeBackground, "background.image"},
		"from the root":           {"background:\n  image: /assets/a.png\n", ErrThemeBackground, "background.image"},
		"this directory":          {"background:\n  image: ./assets/a.png\n", ErrThemeBackground, "background.image"},
		"the directory above":     {"background:\n  image: assets/../a.png\n", ErrThemeBackground, "background.image"},
		"a directory below":       {"background:\n  image: assets/more/a.png\n", ErrThemeBackground, "background.image"},
		"another origin":          {"background:\n  image: https://example.test/assets/a.png\n", ErrThemeBackground, "background.image"},
		"another origin, short":   {"background:\n  image: //example.test/assets/a.png\n", ErrThemeBackground, "background.image"},
		"data":                    {"background:\n  image: data:image/png;base64,AAAA\n", ErrThemeBackground, "background.image"},
		"a query":                 {"background:\n  image: assets/a.png?v=2\n", ErrThemeBackground, "background.image"},
		"a fragment":              {"background:\n  image: \"assets/a.png#part\"\n", ErrThemeBackground, "background.image"},
		"a backslash":             {"background:\n  image: 'assets\\a.png'\n", ErrThemeBackground, "background.image"},
		"percent encoding":        {"background:\n  image: assets/%61.png\n", ErrThemeBackground, "background.image"},
		"a space":                 {"background:\n  image: assets/my photo.png\n", ErrThemeBackground, "background.image"},
		"a hidden file":           {"background:\n  image: assets/.a.png\n", ErrThemeBackground, "background.image"},
		"upper-case directory":    {"background:\n  image: ASSETS/a.png\n", ErrThemeBackground, "background.image"},
		"as in a stylesheet":      {"background:\n  image: url(assets/a.png)\n", ErrThemeBackground, "background.image"},
		"a font":                  {"background:\n  image: assets/brand.woff2\n", ErrThemeBackground, "background.image"},
		"an SVG file":             {"background:\n  image: assets/a.svg\n", ErrThemeBackground, "background.image"},
		"no extension":            {"background:\n  image: assets/a\n", ErrThemeBackground, "background.image"},
		"a number":                {"background:\n  image: 5\n", ErrThemeBackground, "background.image"},
		"a name that is too long": {"background:\n  image: assets/" + strings.Repeat("a", 61) + ".png\n", ErrThemeBackground, "background.image"},
		// The dim is a whole number of percent.
		"below nothing":      {"background:\n  image: assets/a.png\n  dim: -1\n", ErrThemeBackground, "background.dim"},
		"above everything":   {"background:\n  image: assets/a.png\n  dim: 101\n", ErrThemeBackground, "background.dim"},
		"far above":          {"background:\n  image: assets/a.png\n  dim: 6000\n", ErrThemeBackground, "background.dim"},
		"the image is first": {"background:\n  image: a.png\n  dim: 101\n", ErrThemeBackground, "background.image"},
		// A value of another shape is not read as an image or as a number.
		"an image as a list":  {"background:\n  image: [assets/a.png]\n", ErrThemeBackground, "background.image"},
		"an image as a map":   {"background:\n  image: {light: assets/a.png}\n", ErrThemeBackground, "background.image"},
		"an image left empty": {"background:\n  image:\n", ErrThemeBackground, "background.image"},
		"a dim in quotes":     {"background:\n  image: assets/a.png\n  dim: \"60\"\n", ErrThemeBackground, "background.dim"},
		"a dim in percent":    {"background:\n  image: assets/a.png\n  dim: 60%\n", ErrThemeBackground, "background.dim"},
		"a dim as a fraction": {"background:\n  image: assets/a.png\n  dim: 0.6\n", ErrThemeBackground, "background.dim"},
		"a dim with a part":   {"background:\n  image: assets/a.png\n  dim: 60.5\n", ErrThemeBackground, "background.dim"},
		"a dim in words":      {"background:\n  image: assets/a.png\n  dim: half\n", ErrThemeBackground, "background.dim"},
		"a dim in base 16":    {"background:\n  image: assets/a.png\n  dim: 0x3c\n", ErrThemeBackground, "background.dim"},
		"a dim left empty":    {"background:\n  image: assets/a.png\n  dim:\n", ErrThemeBackground, "background.dim"},
		"a dim as a list":     {"background:\n  image: assets/a.png\n  dim: [60]\n", ErrThemeBackground, "background.dim"},
		"a dim as a switch":   {"background:\n  image: assets/a.png\n  dim: true\n", ErrThemeBackground, "background.dim"},
		// A background that is not a mapping, or that says a thing twice, is
		// not a theme file this release can read.
		"a text":         {"background: assets/a.png\n", ErrThemeSyntax, "background: "},
		"a list":         {"background:\n  - image: assets/a.png\n", ErrThemeSyntax, "background: "},
		"an image twice": {"background:\n  image: assets/a.png\n  image: assets/b.png\n", ErrThemeSyntax, "background: "},
		"given twice":    {"background:\n  image: assets/a.png\nbackground:\n  image: assets/b.png\n", ErrThemeSyntax, ""},
	} {
		_, _, err := ReadThemeFile([]byte(validThemeFile + tc.written))
		var rejection *ThemeFileError
		if !errors.As(err, &rejection) {
			t.Errorf("%s: accepted (%v)", name, err)
			continue
		}
		if !errors.Is(err, tc.reason) || !strings.HasPrefix(rejection.Subject, tc.subject) || (tc.reason == ErrThemeBackground && rejection.Subject != tc.subject) {
			t.Errorf("%s: got %v with %q, want %v with %q", name, rejection.Reason, rejection.Subject, tc.reason, tc.subject)
		}
	}

	// The same holds for a theme that is not read from a file.
	valid, _, err := ReadThemeFile([]byte(validThemeFile))
	if err != nil {
		t.Fatal(err)
	}
	for name, background := range map[string]ThemeBackground{
		"no image": {}, "another place": {Image: "wallpaper.jpg"}, "a font": {Image: "assets/a.ttf"},
		"below nothing": {Image: "assets/a.png", Dim: -1}, "above everything": {Image: "assets/a.png", Dim: 101},
	} {
		valid.Background = &background
		if err = valid.Validate(); !errors.Is(err, ErrThemeBackground) {
			t.Errorf("%s: %v", name, err)
		}
	}
	valid.Background = &ThemeBackground{Image: "assets/a.png", Dim: 100}
	if err = valid.Validate(); err != nil {
		t.Fatal(err)
	}
}

// The name of the image is taken from the way it is written.
func TestThemeBackgroundAsset(t *testing.T) {
	for image, want := range map[string]string{
		"assets/a.png": "a.png", "assets/Wall.JPG": "Wall.JPG", "assets/a.b.c.webp": "a.b.c.webp",
	} {
		if name, named := (ThemeBackground{Image: image}).Asset(); !named || name != want {
			t.Errorf("%q: %q %v", image, name, named)
		}
	}
	for _, image := range []string{"", "a.png", "assets/", "assets/a", "assets/a.woff2", "assets/a.ttf", "assets/a.svg", "assets/a.png/", "assets/a/b.png", "/assets/a.png"} {
		if name, named := (ThemeBackground{Image: image}).Asset(); named || name != "" {
			t.Errorf("%q names %q", image, name)
		}
	}
}
