package editor

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode"
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
	if messages["保存更改"] != "Save changes" {
		t.Fatal("missing client translations")
	}
}
