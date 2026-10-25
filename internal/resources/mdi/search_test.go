package mdi

import (
	"reflect"
	"sort"
	"testing"
)

func TestSearchIconNames(t *testing.T) {
	first, total := SearchIconNames("", 0, 48)
	if len(first) != 48 || total <= 48 || !sort.StringsAreSorted(first) {
		t.Fatalf("unexpected first page %d/%d", len(first), total)
	}
	second, _ := SearchIconNames("", 48, 48)
	if first[len(first)-1] >= second[0] {
		t.Fatal("pages overlap or are unordered")
	}
	matches, count := SearchIconNames(" SERVER ", 0, 48)
	lower, _ := SearchIconNames("server", 0, 48)
	if count == 0 || !reflect.DeepEqual(matches, lower) {
		t.Fatal("case-insensitive search failed")
	}
	first[0] = "mutated"
	again, _ := SearchIconNames("", 0, 48)
	if again[0] == "mutated" {
		t.Fatal("caller mutated cached names")
	}
	empty, count := SearchIconNames("not-a-real-icon-name-xyz", 0, 48)
	if len(empty) != 0 || count != 0 {
		t.Fatal("unexpected matches")
	}
	outside, _ := SearchIconNames("", total+1, 48)
	if len(outside) != 0 {
		t.Fatal("out-of-range page must be empty")
	}
}
