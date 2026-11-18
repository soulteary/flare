package server

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
)

// The page for import and export is laid out as the pages of the settings
// are: a section to a subject, each with a title, and every control under a
// label of its own. Choosing the data, exporting and importing are three
// sections, in that order.
func TestTransferPageIsLaidOutInSectionsHTTP(t *testing.T) {
	customThemeDirectory(t, "Locale: en\nTheme: blackboard\n")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true, EnableEditor: true})
	require.NoError(t, err)
	document, err := html.Parse(strings.NewReader(getPage(t, handler, "/settings/transfer")))
	require.NoError(t, err)

	type section struct {
		class, title string
		controls     []string
	}
	var sections []section
	labelled := map[string]string{}
	var titles []string
	walkNodes(document, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		switch node.Data {
		case "h1", "h2":
			titles = append(titles, node.Data+" "+strings.TrimSpace(nodeText(node)))
		case "label":
			// The switch of the menu has a label too; it is not of the page.
			if target := nodeAttribute(node, "for"); target != "" && target != "settings-nav-toggle" {
				require.NotContains(t, labelled, target, "two labels for one control")
				labelled[target] = strings.TrimSpace(nodeText(node))
			}
		case "section":
			found := section{class: nodeAttribute(node, "class")}
			if nodeAttribute(node, "id") == "preview" {
				found.class = "preview"
			}
			walkNodes(node, func(part *html.Node) {
				if part.Type != html.ElementNode {
					return
				}
				switch part.Data {
				case "h3":
					require.Empty(t, found.title, "a section has one title")
					found.title = strings.TrimSpace(nodeText(part))
				case "select", "input", "button":
					if id := nodeAttribute(part, "id"); id != "" {
						found.controls = append(found.controls, id)
					}
				}
			})
			sections = append(sections, found)
		}
	})
	text := func(key string) string { return i18n.T("en", key) }
	require.Equal(t, []section{
		{"transfer-data", text("transfer.choose_data"), []string{"collection"}},
		{"transfer-export", text("transfer.export"), []string{"export-html", "export-yaml", "backup"}},
		{"transfer-import", text("transfer.import_bookmarks"), []string{"file", "format", "strategy", "folder", "flatten", "stripRoot", "allowOther", "import-public", "preview-button"}},
		{"preview", text("transfer.review"), []string{"filter", "prev", "next", "commit", "report"}},
	}, sections)
	// One main title, that of the settings, and the title of the page.
	require.Equal(t, []string{"h1 " + text("app_settings"), "h2 " + text("nav_transfer")}, titles)
	// A control that takes a value stands under its label.
	require.Equal(t, map[string]string{
		"collection": text("transfer.collection"), "file": text("transfer.file"), "format": text("transfer.file_format"),
		"strategy": text("transfer.duplicates"), "folder": text("transfer.target_category"),
	}, labelled)
}
