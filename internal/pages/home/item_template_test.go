package home

import (
	"html/template"
	"strings"
	"testing"

	"github.com/soulteary/flare/internal/themetemplate"
)

// The host of an entry is the one its address leads to on the web, without
// the port, and nothing for any other address.
func TestEntryHost(t *testing.T) {
	for address, want := range map[string]string{
		"https://example.com/":                   "example.com",
		"http://Example.com:8080/a?b=c#d":        "Example.com",
		"https://user:secret@example.com/":       "example.com",
		"https://[2001:db8::1]:8443/":            "2001:db8::1",
		"https://例子.example/":                    "例子.example",
		"":                                       "",
		"example.com":                            "",
		"/local/page":                            "",
		"//example.com/":                         "",
		"mailto:someone@example.com":             "",
		"ftp://example.com/":                     "",
		"chrome-extension://abcdef/page.html":    "",
		"javascript:alert(1)":                    "",
		"https://exa mple.com/":                  "",
		"http://%zz/":                            "",
		"HTTPS://EXAMPLE.COM/":                   "EXAMPLE.COM",
		"https://example.com:notaport/":          "",
		"  https://example.com/":                 "",
		"https:///path":                          "",
		"https://example.com./":                  "example.com.",
		"http://localhost:5005/":                 "localhost",
		"http://192.168.1.10:8096/web/index.htm": "192.168.1.10",
	} {
		if got := entryHost(address); got != want {
			t.Errorf("%q: %q", address, got)
		}
	}
}

// The initial of an entry is the first character of its name as it is
// written, whatever the script, and nothing for a name without one.
func TestEntryInitial(t *testing.T) {
	for name, want := range map[string]string{
		"Alpha": "A", "beta": "b", "  gamma": "g", "\t\n delta ": "d", "数据库": "数", "Ünïcode": "Ü", "1Password": "1",
		"<b>": "<", "😀 smile": "😀", "": "", "   ": "",
	} {
		if got := entryInitial(name); got != want {
			t.Errorf("%q: %q", name, got)
		}
	}
}

// What fills the template of an entry in: each field by its name, the
// address, the attributes the application gives the link, and the icon.
func TestEntryContent(t *testing.T) {
	entry := entryContent{name: "Name", description: "Description", host: "host", initial: "N", href: "https://example.com/", icon: "<svg></svg>"}
	for field, want := range map[string]string{
		themetemplate.FieldName: "Name", themetemplate.FieldDescription: "Description", themetemplate.FieldHost: "host",
		themetemplate.FieldInitial: "N", "Other": "", "": "",
	} {
		if got := entry.Text(field); got != want {
			t.Errorf("%q: %q", field, got)
		}
	}
	if entry.Address() != "https://example.com/" || entry.LinkAttributes() != ` rel="noopener"` {
		t.Errorf("link: %q %q", entry.Address(), entry.LinkAttributes())
	}
	entry.newTab = true
	if entry.LinkAttributes() != ` target="_blank" rel="noopener"` {
		t.Errorf("link in a new window: %q", entry.LinkAttributes())
	}
	if entry.Slot(themetemplate.SlotIcon) != template.HTML("<svg></svg>") || entry.Slot(themetemplate.SlotItems) != "" || entry.Slot("") != "" {
		t.Errorf("parts: %q %q", entry.Slot(themetemplate.SlotIcon), entry.Slot(themetemplate.SlotItems))
	}

	// A group has a name and writes its bookmarks where they are placed.
	var out strings.Builder
	group := groupContent{name: "Group", bookmarks: func() { out.WriteString("<li></li>") }}
	if group.Text(themetemplate.FieldName) != "Group" || group.Address() != "" || group.LinkAttributes() != "" || group.Slot(themetemplate.SlotItems) != "" {
		t.Error("a group has a name and nothing else to hand over")
	}
	group.WriteSlot(&out, themetemplate.SlotIcon)
	if out.Len() != 0 {
		t.Errorf("the bookmarks were written in place of an icon: %q", out.String())
	}
	group.WriteSlot(&out, themetemplate.SlotItems)
	if out.String() != "<li></li>" {
		t.Errorf("bookmarks: %q", out.String())
	}
}
