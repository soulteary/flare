package home

import (
	"strings"
	"testing"

	"github.com/soulteary/flare/internal/fn"
)

func TestNavigationHrefExtensionSchemeUsesValidatedPrefix(t *testing.T) {
	for _, value := range []string{"chrome-extension://abc/page", "CHROME-EXTENSION://abc/page"} {
		if actual := navigationHref(value, false); !strings.HasPrefix(actual, "/redir/url?go=") {
			t.Fatalf("extension scheme was not routed through compatibility helper: %q", actual)
		}
	}
	for _, value := range []string{"javascript:alert(1)", "javascript://chrome-extension:abc", "chrome-extension:\\bad"} {
		if actual := navigationHref(value, false); actual != "#" {
			t.Fatalf("unsafe URL passed validation: %q", actual)
		}
	}
}

func BenchmarkNavigationURLValidation(b *testing.B) {
	b.Run("href", func(b *testing.B) {
		for range b.N {
			navigationHref("https://example.com/path?a=1&b=2#part", false)
		}
	})
	b.Run("remote-image", func(b *testing.B) {
		for range b.N {
			fn.IsSafeImageURL("https://example.com/icon.svg")
		}
	})
	b.Run("builtin-icon", func(b *testing.B) {
		icons := []string{"serverNetwork", "home"}
		for i := range b.N {
			fn.IsSafeImageURL(icons[i%len(icons)])
		}
	})
}
