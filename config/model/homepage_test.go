package model

import "testing"

func TestHomepageLayoutChoices(t *testing.T) {
	for _, value := range []string{"", "single", "compact"} {
		options := &HomepageOptions{MobileBookmarkLayout: value}
		if err := options.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	if (&HomepageOptions{MobileBookmarkLayout: "injected"}).Validate() == nil {
		t.Fatal("invalid layout accepted")
	}
	var absent *HomepageOptions
	if absent.Classes() != "" {
		t.Fatal("old configurations must keep sequential layout")
	}
	if (&HomepageOptions{MobileBookmarkLayout: "compact"}).Classes() != "mobile-bookmarks-compact " {
		t.Fatal("compact class missing")
	}
}
