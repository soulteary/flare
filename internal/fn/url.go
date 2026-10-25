package fn

import (
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// DynamicURL holds parsed request URL components. Use ParseRequestURLTo and ParseDynamicUrlWith to avoid global state.
type DynamicURL struct {
	Host     string
	Hostname string
	Href     string
	Origin   string
	Pathname string
	Port     string
	Protocol string
}

func getPort(host string, defaultPort string) (hostname string, port string) {
	hostname, port = host, defaultPort
	if h, p, err := net.SplitHostPort(host); err == nil {
		return h, p
	}
	// A bracketed IPv6 literal without a port is a valid HTTP authority.
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		hostname = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	return
}

// EncodedNavigationURL carries only the source request's relative URI. The
// redirect endpoint rebuilds authority and scheme from its own request, never
// from caller-provided forwarding headers or an arbitrary source origin.
func EncodedNavigationURL(target string, info *DynamicURL) string {
	link := "/redir/url?go=" + url.QueryEscape(base64.StdEncoding.EncodeToString([]byte(target)))
	if info != nil {
		source := strings.TrimPrefix(info.Href, info.Origin)
		if validSourceURI(source) {
			link += "&from=" + url.QueryEscape(source)
		}
	}
	return link
}

func validSourceURI(source string) bool {
	if !strings.HasPrefix(source, "/") || strings.HasPrefix(source, "//") || strings.ContainsAny(source, "\\\r\n\x00") {
		return false
	}
	parsed, err := url.ParseRequestURI(source)
	return err == nil && !parsed.IsAbs() && parsed.Host == "" && parsed.Fragment == ""
}

// ParseNavigationSource restores the source page context for encoded dynamic
// links. Missing context uses the homepage for compatibility with older links.
func ParseNavigationSource(r *http.Request, source string) (DynamicURL, bool) {
	if source == "" {
		source = "/"
	}
	if !validSourceURI(source) {
		return DynamicURL{}, false
	}
	parsed, err := url.ParseRequestURI(source)
	if err != nil {
		return DynamicURL{}, false
	}
	clone := *r
	clone.URL, clone.RequestURI = parsed, source
	return ParseRequestURLTo(&clone), true
}

// ParseRequestURLTo parses request-local URL metadata without shared state.
func ParseRequestURLTo(r *http.Request) DynamicURL {
	scheme := "http:"
	defaultPort := "80"
	if r != nil && r.TLS != nil {
		scheme = "https:"
		defaultPort = "443"
	}
	host := ""
	if r != nil {
		host = r.Host
	}
	hostname, port := getPort(host, defaultPort)
	pathname := ""
	requestURI := ""
	if r != nil && r.URL != nil {
		pathname = r.URL.Path
		requestURI = r.RequestURI
	}
	return DynamicURL{
		Host:     host,
		Hostname: hostname,
		Href:     strings.Join([]string{scheme, "//", host, requestURI}, ""),
		Origin:   strings.Join([]string{scheme, "//", host}, ""),
		Pathname: pathname,
		Port:     port,
		Protocol: scheme,
	}
}

// ParseDynamicUrlWith substitutes URL placeholders using info. Concurrency-safe when info is request-scoped.
func ParseDynamicUrlWith(url string, info *DynamicURL) string {
	if info == nil {
		return url
	}
	result := url
	result = strings.ReplaceAll(result, "{host}", info.Host)
	result = strings.ReplaceAll(result, "{hostname}", info.Hostname)
	result = strings.ReplaceAll(result, "{href}", info.Href)
	result = strings.ReplaceAll(result, "{origin}", info.Origin)
	result = strings.ReplaceAll(result, "{pathname}", info.Pathname)
	result = strings.ReplaceAll(result, "{port}", info.Port)
	result = strings.ReplaceAll(result, "{protocol}", info.Protocol)
	return result
}
