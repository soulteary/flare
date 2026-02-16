package FlareFn

import (
	"strings"
	"testing"
)

func TestGetYandexFavicon_ValidURL(t *testing.T) {
	const fallback = "https://fallback/favicon.ico"
	out := GetYandexFavicon("https://github.com/soulteary", fallback)
	expected := "https://favicon.yandex.net/favicon/github.com/"
	if !strings.Contains(out, expected) {
		t.Errorf("GetYandexFavicon: expected substring %q in %q", expected, out)
	}
	if !strings.HasPrefix(out, "<img src=") {
		t.Errorf("GetYandexFavicon: expected img tag, got %q", out)
	}
}

func TestGetYandexFavicon_InvalidURL(t *testing.T) {
	const fallback = "https://fallback/favicon.ico"
	out := GetYandexFavicon("://invalid", fallback)
	if out != fallback {
		t.Errorf("GetYandexFavicon invalid URL should return fallback: got %q", out)
	}
}
