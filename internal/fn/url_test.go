package fn

import (
	"crypto/tls"
	"net/http"
	"testing"
)

func TestParseRequestURL_HTTP(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://example.com:8080/foo/bar", nil)
	r.Host = "example.com:8080"
	info := ParseRequestURLTo(r)
	if info.Host != "example.com:8080" {
		t.Errorf("Host: got %q", info.Host)
	}
	if info.Hostname != "example.com" {
		t.Errorf("Hostname: got %q", info.Hostname)
	}
	if info.Port != "8080" {
		t.Errorf("Port: got %q", info.Port)
	}
	if info.Protocol != "http:" {
		t.Errorf("Protocol: got %q", info.Protocol)
	}
	if info.Pathname != "/foo/bar" {
		t.Errorf("Pathname: got %q", info.Pathname)
	}
}

func TestParseRequestURL_HTTPS(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	r.Host = "example.com"
	r.TLS = &tls.ConnectionState{}
	info := ParseRequestURLTo(r)
	if info.Protocol != "https:" {
		t.Errorf("Protocol: got %q", info.Protocol)
	}
	if info.Port != "443" {
		t.Errorf("Port: got %q", info.Port)
	}
}

func TestParseRequestURL_NoPort(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	r.Host = "example.com"
	info := ParseRequestURLTo(r)
	if info.Hostname != "example.com" || info.Port != "80" {
		t.Errorf("Hostname=%q Port=%q", info.Hostname, info.Port)
	}
}

func TestParseDynamicUrl(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://localhost:5005/", nil)
	r.Host = "localhost:5005"
	info := ParseRequestURLTo(r)
	out := ParseDynamicUrlWith("origin={origin} host={host} path={pathname}", &info)
	if out != "origin=http://localhost:5005 host=localhost:5005 path=/" {
		t.Errorf("ParseDynamicUrl: got %q", out)
	}
	out2 := ParseDynamicUrlWith("no placeholders", &info)
	if out2 != "no placeholders" {
		t.Errorf("ParseDynamicUrl no placeholders: got %q", out2)
	}
}

func TestParseRequestURLToIPv6(t *testing.T) {
	for _, tc := range []struct{ host, hostname, port string }{
		{"[2001:db8::1]:5005", "2001:db8::1", "5005"},
		{"[2001:db8::1]", "2001:db8::1", "80"},
		{"2001:db8::1", "2001:db8::1", "80"},
		{"example.com:1234", "example.com", "1234"},
		{"example.com", "example.com", "80"},
	} {
		t.Run(tc.host, func(t *testing.T) {
			r := &http.Request{Host: tc.host}
			info := ParseRequestURLTo(r)
			if info.Hostname != tc.hostname || info.Port != tc.port {
				t.Fatalf("hostname=%q port=%q", info.Hostname, info.Port)
			}
		})
	}
}

func TestNavigationSourceValidation(t *testing.T) {
	request, _ := http.NewRequest(http.MethodGet, "http://example.com/redir/url", nil)
	for _, source := range []string{"https://other.example/", "//other.example/", "/bad\\path", "/bad\r\npath"} {
		if _, ok := ParseNavigationSource(request, source); ok {
			t.Errorf("accepted foreign or malformed source %q", source)
		}
	}
	info, ok := ParseNavigationSource(request, "/bookmarks?search=hello%20world")
	if !ok || info.Pathname != "/bookmarks" || info.Href != "http://example.com/bookmarks?search=hello%20world" {
		t.Fatalf("source=%+v valid=%v", info, ok)
	}
}
