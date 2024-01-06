package FlareFn

import (
	"net/http"
	"regexp"
	"strings"
)

type _Dynamic_URL struct {
	Host     string
	Hostname string
	Href     string
	Origin   string
	Pathname string
	Port     string
	Protocol string
}

var RequestURL _Dynamic_URL

func getPort(host string, defaultPort string) (hostname string, port string) {
	hostname = host
	port = defaultPort
	reg, _ := regexp.Compile(`([\w+\.-]+):(\d+)$`)
	portMatch := reg.FindStringSubmatch(host)
	if portMatch != nil {
		hostname = portMatch[1]
		port = portMatch[2]
	}
	return
}

func ParseRequestURL(r *http.Request) {
	scheme := "http:"
	defaultPort := "80"
	if r.TLS != nil {
		scheme = "https:"
		defaultPort = "443"
	}
	host := r.Host
	hostname, port := getPort(host, defaultPort)

	RequestURL.Host = host
	RequestURL.Hostname = hostname
	RequestURL.Href = strings.Join([]string{scheme, "//", host, r.RequestURI}, "")
	RequestURL.Origin = strings.Join([]string{scheme, "//", host}, "")
	RequestURL.Pathname = r.URL.Path
	RequestURL.Port = port
	RequestURL.Protocol = scheme
}

func ParseDynamicUrl(url string) string {
	result := url
	result = strings.ReplaceAll(result, "{host}", RequestURL.Host)
	result = strings.ReplaceAll(result, "{hostname}", RequestURL.Hostname)
	result = strings.ReplaceAll(result, "{href}", RequestURL.Href)
	result = strings.ReplaceAll(result, "{origin}", RequestURL.Origin)
	result = strings.ReplaceAll(result, "{pathname}", RequestURL.Pathname)
	result = strings.ReplaceAll(result, "{port}", RequestURL.Port)
	result = strings.ReplaceAll(result, "{protocol}", RequestURL.Protocol)
	return result
}
