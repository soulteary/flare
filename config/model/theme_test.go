package model

import (
	"errors"
	"math"
	"testing"
)

func TestPaletteValidate(t *testing.T) {
	valid := Palette{Background: "#10241b", Primary: "#e8f1e4", Accent: "#f2b84b"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid palette rejected: %v", err)
	}
	// Each case replaces one color of the valid palette.
	for _, value := range []string{
		"", "#fff", "red", "10241b", "#10241", "#10241bb", "#10241g", " #10241b", "rgb(0,0,0)",
		"#fff;x:y", "#10241b;background:url(//example.test/)", "var(--x)", "#１０２４１ｂ",
	} {
		for field, palette := range map[string]Palette{
			"background": {Background: value, Primary: valid.Primary, Accent: valid.Accent},
			"primary":    {Background: valid.Background, Primary: value, Accent: valid.Accent},
			"accent":     {Background: valid.Background, Primary: valid.Primary, Accent: value},
		} {
			if err := palette.Validate(); !errors.Is(err, ErrPaletteColor) {
				t.Errorf("%s %q: got %v, want ErrPaletteColor", field, value, err)
			}
		}
	}
}

func TestPaletteValidateRequiresReadablePrimary(t *testing.T) {
	// On white, #949494 is 3.03:1 and #959595 is 2.995:1: the nearest grays on
	// either side of the threshold.
	accepted := Palette{Background: "#ffffff", Primary: "#949494", Accent: "#000000"}
	if err := accepted.Validate(); err != nil {
		t.Fatalf("contrast %.2f rejected: %v", ContrastRatio(accepted.Primary, accepted.Background), err)
	}
	rejected := Palette{Background: "#ffffff", Primary: "#959595", Accent: "#000000"}
	if err := rejected.Validate(); !errors.Is(err, ErrPaletteContrast) {
		t.Fatalf("contrast %.2f: got %v, want ErrPaletteContrast", ContrastRatio(rejected.Primary, rejected.Background), err)
	}
	// The accent color is decorative and is not held to the threshold.
	lowAccent := Palette{Background: "#ffffff", Primary: "#222222", Accent: "#fefefe"}
	if err := lowAccent.Validate(); err != nil {
		t.Fatalf("low-contrast accent rejected: %v", err)
	}
}

func TestPaletteNormalized(t *testing.T) {
	got := Palette{Background: " #FFFDEA ", Primary: "#AbCdEf", Accent: "#000000\n"}.Normalized()
	want := Palette{Background: "#fffdea", Primary: "#abcdef", Accent: "#000000"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestContrastRatio(t *testing.T) {
	near := func(got, want float64) bool { return math.Abs(got-want) < 0.005 }
	if got := ContrastRatio("#000000", "#ffffff"); !near(got, 21) {
		t.Errorf("black on white: %.3f", got)
	}
	if got := ContrastRatio("#FFFFFF", "#000000"); !near(got, 21) {
		t.Errorf("white on black: %.3f", got)
	}
	if got := ContrastRatio("#336699", "#336699"); !near(got, 1) {
		t.Errorf("identical colors: %.3f", got)
	}
	// Reference value from the WCAG 2 definition: #767676 on white is 4.54:1.
	if got := ContrastRatio("#767676", "#ffffff"); !near(got, 4.54) {
		t.Errorf("#767676 on white: %.3f", got)
	}
	for _, pair := range [][2]string{{"", "#ffffff"}, {"#ffffff", "white"}, {"#fff", "#000"}} {
		if got := ContrastRatio(pair[0], pair[1]); got != 0 {
			t.Errorf("%q against %q: %.3f, want 0", pair[0], pair[1], got)
		}
	}
}
