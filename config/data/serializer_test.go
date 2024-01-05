package FlareData

import (
	"testing"
)

func TestJSONStringify(t *testing.T) {
	type testJSON struct {
		Project string `json:"project"`
	}
	const src = "{\"project\":\"flare\"}"
	dest := jsonStringify(testJSON{Project: "flare"})

	if src != dest {
		t.Fatal("JSON Stringify Error")
	}

	// mock incorrect data
	errTest := jsonStringify(make(chan int))
	if errTest != "{}" {
		t.Fatal("JSON Stringify Error")
	}
}

func TestMaskTextWithStars(t *testing.T) {
	if MaskTextWithStars("1234") != "1**4" {
		t.Fatal("MaskTextWithStars Error")
	}

	if MaskTextWithStars("123") != "1*3" {
		t.Fatal("MaskTextWithStars Error")
	}

	if MaskTextWithStars("12") != "12" {
		t.Fatal("MaskTextWithStars Error")
	}

	if MaskTextWithStars("1") != "1" {
		t.Fatal("MaskTextWithStars Error")
	}

	if MaskTextWithStars("") != "" {
		t.Fatal("MaskTextWithStars Error")
	}
}

func TestGenerateRandomString(t *testing.T) {
	name1 := GenerateRandomString(8)
	if len(name1) != 8 {
		t.Fatal("GenerateRandomString length error")
	}
}
