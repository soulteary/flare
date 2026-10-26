package editor

import (
	"bytes"
	"html/template"
	"os"
	"os/exec"
	"strings"
	"testing"
	"unicode"

	"github.com/soulteary/flare/internal/bookmarks"
	"github.com/soulteary/flare/internal/i18n"
)

func TestTransferTemplateLanguagesAndClientSyntax(t *testing.T) {
	tmpl, err := template.New("transfer.html").Funcs(template.FuncMap{"T": i18n.T}).ParseFiles("../../../embed/templates/transfer.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, locale := range []string{"zh", "en"} {
		var output bytes.Buffer
		if err := tmpl.Execute(&output, map[string]string{"Locale": locale, "HTMLLang": i18n.HTMLLang(locale)}); err != nil {
			t.Fatal(err)
		}
		body := output.String()
		if !strings.Contains(body, `lang="`+i18n.HTMLLang(locale)+`"`) {
			t.Fatal("missing document language")
		}
		if locale == "en" && strings.ContainsFunc(body, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
			t.Fatal("English transfer page contains untranslated Chinese")
		}
		if strings.Contains(body, "ZgotmplZ") {
			t.Fatal("template contains an unsafe context substitution")
		}
		node, err := exec.LookPath("node")
		if err != nil {
			continue
		}
		start := strings.Index(body, "<script>") + len("<script>")
		end := strings.Index(body[start:], "</script>") + start
		command := exec.Command(node, "--check", "-")
		command.Stdin = strings.NewReader(body[start:end])
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%s script syntax: %v\n%s", locale, err, out)
		}
	}
}

func TestTransferReasonsTranslateWithoutChangingUserData(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("config.yml", []byte("Locale: en\n"), 0600); err != nil {
		t.Fatal(err)
	}
	plan := bookmarks.Plan{Rows: []bookmarks.Row{{Name: "我的书签", Target: "我的分类", Reason: "不支持或不安全的协议: javascript"}, {Reason: "缺少名称，已使用域名或链接"}}}
	localizeTransferPlan(&plan)
	if plan.Rows[0].Reason != "Unsupported or unsafe protocol: javascript" {
		t.Fatalf("unexpected reason: %q", plan.Rows[0].Reason)
	}
	if plan.Rows[0].Name != "我的书签" || plan.Rows[0].Target != "我的分类" {
		t.Fatal("user data changed during translation")
	}
	if plan.Rows[1].Reason != "Missing name; hostname or URL used instead." {
		t.Fatal("missing-name reason untranslated")
	}
	if got := transferText("open /private/example/config.yml: permission denied"); strings.Contains(got, "/private/") {
		t.Fatal("internal diagnostic exposed")
	}
}
