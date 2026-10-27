package auth

import (
	"html"
	"net/url"
	"strings"

	nethtml "golang.org/x/net/html"
)

// LoginFooter is isolated from the legacy trusted HTML Footer. Only text and
// explicit HTTP(S) links are emitted; images, scripts and style are discarded.
func loginFooter(input string) string {
	if input == "" {
		return ""
	}
	nodes, err := nethtml.ParseFragment(strings.NewReader(input), nil)
	if err != nil {
		return ""
	}
	var out strings.Builder
	var walk func(*nethtml.Node)
	walk = func(n *nethtml.Node) {
		if n.Type == nethtml.TextNode {
			out.WriteString(html.EscapeString(n.Data))
			return
		}
		if n.Type == nethtml.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "iframe" || n.Data == "object" || n.Data == "template") {
			return
		}
		link := ""
		if n.Type == nethtml.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					u, e := url.Parse(attr.Val)
					if e == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil {
						link = u.String()
					}
				}
			}
			if link != "" {
				out.WriteString(`<a rel="noopener noreferrer" href="` + html.EscapeString(link) + `">`)
			}
		}
		if n.Type == nethtml.ElementNode && n.Data == "br" {
			out.WriteString("<br>")
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if link != "" {
			out.WriteString("</a>")
		}
		if n.Type == nethtml.ElementNode && n.Data == "p" {
			out.WriteString("<br>")
		}
	}
	for _, node := range nodes {
		walk(node)
	}
	return `<footer>` + out.String() + `</footer>`
}
