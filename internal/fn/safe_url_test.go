package fn

import "testing"

func TestNavigationURLPolicy(t *testing.T) {
	for _, raw := range []string{"https://nas.example:8443/a?x=1&y=2#part", "http://[::1]:8080/", "/relative", "./relative", "relative", "//nas.example/panel", "mailto:user@example.com", "ssh://nas", "smb://nas/share", "magnet:?xt=urn:btih:abc", "steam://run/123", "obsidian://open?vault=test", "chrome-extension://abc/panel"} {
		if !IsSafeNavigationURL(raw) {
			t.Errorf("supported navigation URL rejected: %q", raw)
		}
	}
	for _, raw := range []string{"", "javascript:alert(1)", "JaVaScRiPt:alert(1)", "data:text/html,test", "vbscript:msgbox(1)", "file:///etc/passwd", "blob:https://example.com/id", "http:example.com", "https:///path", "java\nscript:alert(1)", "\\\\example.com/path", "https://example.com/\x00"} {
		if IsSafeNavigationURL(raw) {
			t.Errorf("unsafe navigation URL accepted: %q", raw)
		}
	}
	for _, raw := range []string{"data:image/svg+xml,test", "javascript:alert(1)", "chrome-extension://abc/icon", "/local.png", "https:///missing-host"} {
		if IsSafeImageURL(raw) {
			t.Errorf("unsafe image URL accepted: %q", raw)
		}
	}
	if !IsSafeImageURL("HTTPS://icons.example/path?a=1&b=2") {
		t.Fatal("case-insensitive HTTPS rejected")
	}
}

func TestSafeImageURLWebPrefixDoesNotBroadenValidation(t *testing.T) {
	for _, value := range []string{"HTTP://example.com/icon.svg", "HTTPS://example.com/icon.svg"} {
		if !IsSafeImageURL(value) {
			t.Errorf("valid web image rejected: %s", value)
		}
	}
	for _, value := range []string{"http:opaque", "https:///missing-host", "http://x/\n", "http://x/\\bad", "http://%zz", "//example.com/icon.svg", "serverNetwork", "data:image/svg+xml,icon"} {
		if IsSafeImageURL(value) {
			t.Errorf("invalid image accepted: %q", value)
		}
	}
}
