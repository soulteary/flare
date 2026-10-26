package editor

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"unicode"

	"github.com/soulteary/flare/internal/i18n"
)

func TestEditorMessagesHaveCompleteEnglishAndChineseBundles(t *testing.T) {
	for source := range editorMessages {
		if got := editorText("zh", source); got != source {
			t.Fatalf("Chinese message changed: %q => %q", source, got)
		}
		english := editorText("en", source)
		if strings.ContainsFunc(english, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
			t.Errorf("English translation missing for %q: %q", source, english)
		}
	}
	var messages map[string]string
	if err := json.Unmarshal([]byte(editorMessageJSON("en")), &messages); err != nil {
		t.Fatal(err)
	}
	if messages["editor.save_changes"] != "Save changes" {
		t.Fatal("missing client translations")
	}
}

func TestEditorCompositeMessagesAreTranslated(t *testing.T) {
	for _, key := range editorCompositeMessages {
		for _, locale := range []string{"zh", "en"} {
			if got := i18n.T(locale, key); got == key || got == "" {
				t.Errorf("missing %s translation: %s", locale, key)
			}
		}
	}
}

func TestEditorClientFormatsNamedParametersOnce(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	script := `
 const fs = require('node:fs'), vm = require('node:vm'), assert = require('node:assert/strict');
 const messages = {move:'Moved {location}: {error}', count:'{count} entries'};
 const context = {window:{},document:{getElementById(){return {dataset:{messages:JSON.stringify(messages)}}}}};
 vm.runInNewContext(fs.readFileSync('client/i18n.js','utf8'),context);
 const t = context.window.FlareEditorI18n.t;
 assert.equal(t('move',{location:'<img> {error} & "x"',error:'failed'}),'Moved <img> {error} & "x": failed');
 assert.equal(t('count',{count:0}),'0 entries');
 assert.equal(t('move',{location:'x'}),'Moved x: {error}');
 assert.equal(t('missing'), 'missing');
 assert.equal(t('toString'), 'toString');
 `
	if out, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("client formatter: %v\n%s", err, out)
	}
}
