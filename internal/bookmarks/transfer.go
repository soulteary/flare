package bookmarks

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/soulteary/flare/config/model"
	xhtml "golang.org/x/net/html"
	"gopkg.in/yaml.v2"
)

// Parse reads browser HTML or the existing Flare YAML schema.
func Parse(raw []byte, format string) ([]Entry, error) {
	if len(raw) > MaxBytes || !utf8.Valid(raw) {
		return nil, fmt.Errorf("文件必须为 UTF-8，且不超过 8 MiB")
	}
	switch format {
	case "html":
		return parseHTML(raw)
	case "yaml":
		var src model.Bookmarks
		if err := yaml.UnmarshalStrict(raw, &src); err != nil {
			return nil, fmt.Errorf("无效的 Flare YAML: %w", err)
		}
		if src.Items == nil {
			return nil, fmt.Errorf("缺少 links 字段")
		}
		if len(src.Items) > MaxItems || len(src.Categories) > MaxItems {
			return nil, fmt.Errorf("最多支持 %d 个书签或分类", MaxItems)
		}
		cats := map[string]string{}
		for _, cat := range src.Categories {
			if cat.ID == "" || strings.TrimSpace(cat.Name) == "" || cats[cat.ID] != "" {
				return nil, fmt.Errorf("分类 ID 或名称无效或重复")
			}
			cats[cat.ID] = cat.Name
		}
		entries := make([]Entry, 0, len(src.Items))
		for _, b := range src.Items {
			e := Entry{Bookmark: b}
			if b.Category != "" {
				name, ok := cats[b.Category]
				if !ok {
					return nil, fmt.Errorf("书签引用了不存在的分类: %s", b.Category)
				}
				e.Path = []string{name}
			}
			entries = append(entries, e)
		}
		return entries, nil
	default:
		return nil, fmt.Errorf("请选择 HTML 或 Flare YAML 格式")
	}
}

func parseHTML(raw []byte) ([]Entry, error) {
	z := xhtml.NewTokenizer(bytes.NewReader(raw))
	var entries []Entry
	var stack []string
	var pending, text, href string
	var inHeading, inLink, sawDL bool
	for {
		kind := z.Next()
		if kind == xhtml.ErrorToken {
			if z.Err() != io.EOF {
				return nil, z.Err()
			}
			if !sawDL || inLink || inHeading || len(stack) != 0 {
				return nil, fmt.Errorf("书签 HTML 不完整或缺少 DL 目录")
			}
			return entries, nil
		}
		switch kind {
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			token := z.Token()
			switch token.Data {
			case "h3":
				inHeading = true
				text = ""
			case "a":
				if inLink {
					return nil, fmt.Errorf("嵌套的书签链接")
				}
				inLink = true
				text = ""
				href = ""
				for _, a := range token.Attr {
					if a.Key == "href" {
						href = a.Val
						break
					}
				}
			case "dl":
				sawDL = true
				stack = append(stack, pending)
				pending = ""
				if len(stack) > MaxDepth+1 {
					return nil, fmt.Errorf("目录深度不能超过 %d", MaxDepth)
				}
			}
		case xhtml.TextToken:
			if inHeading || inLink {
				text += string(z.Text())
			}
		case xhtml.EndTagToken:
			token := z.Token()
			switch token.Data {
			case "h3":
				if inHeading {
					pending = strings.TrimSpace(text)
					inHeading = false
				}
			case "a":
				if !inLink {
					continue
				}
				path := make([]string, 0, len(stack))
				for _, part := range stack {
					if part != "" {
						path = append(path, part)
					}
				}
				entries = append(entries, Entry{Bookmark: model.Bookmark{Name: strings.TrimSpace(text), URL: href}, Path: path})
				if len(entries) > MaxItems {
					return nil, fmt.Errorf("最多支持 %d 个书签", MaxItems)
				}
				inLink = false
			case "dl":
				if len(stack) == 0 {
					return nil, fmt.Errorf("目录结构不完整")
				}
				stack = stack[:len(stack)-1]
				pending = ""
			}
		}
	}
}

// URLKey validates a link without fetching it; normalization is used only for comparison.
func URLKey(raw string, allowOther bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\\") || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("链接为空或包含非法字符")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("链接格式无效")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme == "http" || scheme == "https" {
		if u.Hostname() == "" || u.Opaque != "" {
			return "", fmt.Errorf("缺少域名")
		}
	} else {
		safe := scheme == "" || scheme == "ftp" || scheme == "ftps" || scheme == "mailto" || scheme == "tel" || scheme == "ssh" || scheme == "smb" || scheme == "rdp" || scheme == "vnc" || scheme == "magnet" || scheme == "steam" || scheme == "obsidian"
		if !safe {
			return "", fmt.Errorf("不支持或不安全的协议: %s", scheme)
		}
		if !allowOther {
			return "", fmt.Errorf("相对地址或其他协议需要手动允许")
		}
	}
	u.Scheme = scheme
	u.Host = strings.ToLower(u.Host)
	return u.String(), nil
}

// Build creates a deterministic merge plan without changing current or entries.
func Build(current model.Bookmarks, entries []Entry, options Options) (Plan, error) {
	p := Plan{Result: model.Bookmarks{Categories: append([]model.Category(nil), current.Categories...), Items: append([]model.Bookmark(nil), current.Items...)}, Rows: []Row{}}
	if options.Strategy != "skip" && options.Strategy != "update" && options.Strategy != "keep" {
		return p, fmt.Errorf("无效的重复处理策略")
	}
	if len(entries) > MaxItems || len(current.Items) > MaxItems {
		return p, fmt.Errorf("书签数量超出限制")
	}
	byName := map[string]string{}
	ids := map[string]bool{}
	existing := map[string]int{}
	for _, c := range current.Categories {
		if _, ok := byName[c.Name]; !ok {
			byName[c.Name] = c.ID
		}
		ids[c.ID] = true
	}
	for i, b := range current.Items {
		if key, err := URLKey(b.URL, true); err == nil {
			k := b.Category + "\x00" + key
			if _, ok := existing[k]; !ok {
				existing[k] = i
			}
		}
	}
	for _, e := range entries {
		b := e.Bookmark
		b.URL = strings.TrimSpace(b.URL)
		path := e.Path
		if options.StripRoot && len(path) > 0 {
			path = path[1:]
		}
		target := strings.Join(path, " / ")
		if options.Flatten || target == "" {
			target = strings.TrimSpace(options.Folder)
			if target == "" {
				target = "导入书签"
			}
		}
		if options.Apps {
			target = "首页应用"
		}
		row := Row{Source: strings.Join(e.Path, " / "), Target: target, Name: b.Name, URL: b.URL}
		key, err := URLKey(b.URL, options.AllowOther)
		if err == nil && b.Icon != "" {
			// Icon identifiers and safe image paths are retained; executable image URLs are rejected.
			if strings.Contains(b.Icon, ":") || strings.HasPrefix(b.Icon, "//") {
				_, err = URLKey(b.Icon, false)
			}
			if strings.IndexFunc(b.Icon, unicode.IsControl) >= 0 {
				err = fmt.Errorf("图标包含非法字符")
			}
		}
		if err != nil {
			row.Action = "invalid"
			row.Reason = err.Error()
			p.Invalid++
			p.Rows = append(p.Rows, row)
			continue
		}
		if strings.TrimSpace(b.Name) == "" {
			u, _ := url.Parse(b.URL)
			b.Name = u.Hostname()
			if b.Name == "" {
				b.Name = b.URL
			}
			row.Name = b.Name
			row.Reason = "缺少名称，已使用域名或链接"
		}
		b.Category = ""
		if !options.Apps {
			id, ok := byName[target]
			if !ok {
				for n := len(p.Result.Categories) + 1; ; n++ {
					id = fmt.Sprintf("import-%d", n)
					if !ids[id] {
						break
					}
				}
				byName[target] = id
				ids[id] = true
				p.Result.Categories = append(p.Result.Categories, model.Category{ID: id, Name: target})
				p.Categories++
			}
			b.Category = id
		}
		k := b.Category + "\x00" + key
		index, duplicate := existing[k]
		switch {
		case duplicate && options.Strategy == "skip":
			row.Action = "skip"
			p.Skipped++
		case duplicate && options.Strategy == "update":
			dest := p.Result.Items[index]
			dest.Name = b.Name
			if b.Icon != "" {
				dest.Icon = b.Icon
			}
			if b.Desc != "" {
				dest.Desc = b.Desc
			}
			// Import never weakens a pre-existing private flag.
			dest.Private = dest.Private || b.Private
			p.Result.Items[index] = dest
			row.Action = "update"
			p.Updated++
		default:
			existing[k] = len(p.Result.Items)
			p.Result.Items = append(p.Result.Items, b)
			row.Action = "add"
			p.Added++
		}
		p.Rows = append(p.Rows, row)
	}
	if len(p.Result.Items) > MaxItems || len(p.Result.Categories) > MaxItems {
		return Plan{}, fmt.Errorf("合并后最多支持 %d 个书签或分类", MaxItems)
	}
	return p, nil
}

// HTML exports a Netscape bookmark document with escaped names and safe links.
func HTML(src model.Bookmarks) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("<!DOCTYPE NETSCAPE-Bookmark-file-1>\n<META HTTP-EQUIV=\"Content-Type\" CONTENT=\"text/html; charset=UTF-8\">\n<TITLE>Flare Bookmarks</TITLE>\n<H1>Flare Bookmarks</H1>\n<DL><p>\n")
	write := func(b model.Bookmark) {
		// Do not export executable URLs from legacy, manually edited data.
		if _, err := URLKey(b.URL, true); err != nil {
			return
		}
		fmt.Fprintf(&out, "<DT><A HREF=\"%s\">%s</A>\n", html.EscapeString(b.URL), html.EscapeString(b.Name))
	}
	// Group once so large exports remain linear in the number of bookmarks.
	grouped := make(map[string][]model.Bookmark)
	for _, b := range src.Items {
		grouped[b.Category] = append(grouped[b.Category], b)
	}
	categories := map[string]bool{}
	for _, c := range src.Categories {
		fmt.Fprintf(&out, "<DT><H3>%s</H3>\n<DL><p>\n", html.EscapeString(c.Name))
		for _, b := range grouped[c.ID] {
			write(b)
		}
		out.WriteString("</DL><p>\n")
		categories[c.ID] = true
	}
	for _, b := range src.Items {
		if !categories[b.Category] {
			write(b)
		}
	}
	out.WriteString("</DL><p>\n")
	return out.Bytes(), nil
}
