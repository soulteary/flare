package mdi

import (
	"net/http"
	"strings"
	"testing"
)

func TestLegacyIconsRemainUsableAfterUpgrade(t *testing.T) {
	for _, inline := range []bool{false, true} {
		app := newIconTestRouter(t, inline)
		for name, oldPath := range legacyIconMap {
			// Existing files commonly use mixed-case MDI identifiers.
			input := strings.ToUpper(name)
			expected := iconMap[name]
			if expected == "" {
				expected = oldPath
			}
			if svg := GetIconSVGByName(input); !strings.Contains(svg, `d="`+expected+`"`) {
				t.Fatalf("legacy preview lost its path: %s", name)
			}
			markup := GetIconByName(input)
			if inline {
				if !strings.Contains(markup, `d="`+expected+`"`) {
					t.Fatalf("legacy inline icon missing: %s", name)
				}
			} else {
				url, err := iconURL(markup)
				if err != nil {
					t.Fatalf("legacy image missing: %s: %v", name, err)
				}
				response := requestIcon(app, http.MethodGet, url, "", "")
				if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `d="`+expected+`"`) {
					t.Fatalf("legacy image HTTP request failed: %s", name)
				}
			}
			names, _ := SearchIconNames(input, 0, 48)
			found := false
			for _, result := range names {
				found = found || result == name
			}
			if !found {
				t.Fatalf("legacy icon cannot be selected in editor: %s", name)
			}
		}
	}
}
