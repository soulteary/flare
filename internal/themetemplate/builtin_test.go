package themetemplate

import (
	"html/template"
	"testing"
)

func builtinSet(t *testing.T, skeleton string) *template.Template {
	t.Helper()
	return template.Must(template.New("page").Parse(`{{define "flare/a"}}A{{end}}{{define "flare/b"}}B{{end}}{{define "skeleton"}}` + skeleton + `{{end}}`))
}

// A template of the application is taken as it wrote: the fragment renders
// the same bytes with the parts in place, in a spelling a theme could not use.
func TestBuiltinKeepsWhatTheTemplateWrote(t *testing.T) {
	fragment, err := Builtin(builtinSet(t, `<div id=one class=x>{{template "flare/a"}}<script>s()</script>{{if .}}never{{end}}{{template "flare/b"}} </div>`), "skeleton", []string{"flare/a", "flare/b"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	holes := fragment.Holes()
	if len(holes) != 2 || holes[0] != (Hole{HoleSlot, "flare/a"}) || holes[1] != (Hole{HoleSlot, "flare/b"}) {
		t.Fatalf("holes: %v", holes)
	}
	got := render(fragment, entry{slots: map[string]template.HTML{"flare/a": "<i>first</i>", "flare/b": "<i>second</i>"}})
	want := `<div id=one class=x><i>first</i><script>s()</script><i>second</i> </div>`
	if got != want {
		t.Fatalf("rendered: %s", got)
	}
	if fragment.Size() != len(want)-len("<i>first</i><i>second</i>") {
		t.Fatalf("size %d", fragment.Size())
	}
}

func TestBuiltinNeedsEachPartOnce(t *testing.T) {
	cases := []struct {
		name, skeleton string
		reason         Reason
		subject        string
	}{
		{"a part is left out", `{{template "flare/a"}}`, ReasonSlotMissing, "flare/b"},
		{"a part is placed twice", `{{template "flare/a"}}{{template "flare/b"}}{{template "flare/a"}}`, ReasonSlotRepeated, "flare/a"},
		{"the template fails", `{{template "flare/a"}}{{template "flare/b"}}{{template "none"}}`, ReasonExecute, ""},
	}
	for _, tc := range cases {
		_, err := Builtin(builtinSet(t, tc.skeleton), "skeleton", []string{"flare/a", "flare/b"}, nil)
		reason, subject := reasonOf(t, err)
		if reason != tc.reason || (tc.subject != "" && subject != tc.subject) {
			t.Fatalf("%s: %s %s", tc.name, reason, subject)
		}
	}
	// A name that is no template is refused where it is defined.
	_, err := Builtin(builtinSet(t, `x`), "skeleton", []string{"{{"}, nil)
	if reason, _ := reasonOf(t, err); reason != ReasonSlotMissing && reason != ReasonExecute {
		t.Fatalf("reason %s", reason)
	}
}
