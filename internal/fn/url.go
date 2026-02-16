package fn

import (
	"net/http"
	"regexp"
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

// RequestURL is the package-level parsed URL (set by ParseRequestURL). Prefer ParseRequestURLTo and passing *DynamicURL for concurrency-safe use.
var RequestURL DynamicURL

var hostPortRe = regexp.MustCompile(`([\w+\.-]+):(\d+)$`)

func getPort(host string, defaultPort string) (hostname string, port string) {
	hostname = host
	port = defaultPort
	portMatch := hostPortRe.FindStringSubmatch(host)
	if portMatch != nil {
		hostname = portMatch[1]
		port = portMatch[2]
	}
	return
}

// ParseRequestURLTo parses r into a DynamicURL without using package-level state. Prefer this over ParseRequestURL when possible.
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

// ParseRequestURL parses r and updates package-level RequestURL. For new code, prefer ParseRequestURLTo and ParseDynamicUrlWith.
func ParseRequestURL(r *http.Request) {
	RequestURL = ParseRequestURLTo(r)
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

func ParseDynamicUrl(url string) string {
	return ParseDynamicUrlWith(url, &RequestURL)
}
