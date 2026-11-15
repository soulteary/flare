// Package themetemplate compiles a template of a theme into text with holes.
//
// A template of a theme is not run for a request. It is run once, with
// placeholders in place of the fields it can show, and what it writes is read
// as a browser reads it. Every placeholder has to stand where the field can
// be filled in safely, every element and attribute has to be one a template
// can hold, and the result is written out again in one spelling. What comes
// of it is a Fragment: text that does not change, and holes that the
// application fills in for each rendering.
//
// The values of an instance therefore never pass through the code of a
// template. A template can place a field and branch on what it is given, and
// what it is given are placeholders, booleans and options. It cannot read a
// name or an address, and it cannot send one anywhere.
package themetemplate

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"html/template"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// ReservedPrefix begins the names of the parts the application renders. A
// template of a theme calls them and cannot define one.
const ReservedPrefix = "flare/"

// Slot is a part of the application that a template can place with
// {{template "<name>"}}.
type Slot struct {
	// Name begins with ReservedPrefix.
	Name string
	// Required parts have to be placed, once. Others can be left out.
	Required bool
	// Inline parts can stand inside a link, as an icon does.
	Inline bool
	// Within names the elements the part can stand directly in. A part
	// without any can stand in every element and beside the others.
	Within []string
}

// Spec says what a template can show and what it has to hold.
type Spec struct {
	// Texts names the fields that are shown as text.
	Texts []string
	// RequiredTexts are fields that have to stand in the content of an
	// element at least once, so that they are shown and read out.
	RequiredTexts []string
	// Link says that the template is about one address: it has exactly one
	// link whose href is the address.
	Link bool
	// Slots are the parts the template can place.
	Slots []Slot
	// Funcs are the functions the template can call besides the built-in
	// ones. What they return is part of the text that does not change.
	Funcs template.FuncMap
	// AssetPrefix begins the address of every asset of the theme. An
	// address that loads something has to be this prefix and the name of a
	// file. Without it no such address is accepted.
	AssetPrefix string
	// Limit is the number of bytes the template can write.
	Limit int
	// Root names the one element the template writes: everything else it
	// writes is inside that element. Without it a template can write
	// several elements and text beside them.
	Root string
	// Elements are elements the template has to write at least once.
	Elements []string
}

// Fields hands out the placeholders for the model a template is run with.
type Fields struct {
	prefix string
	texts  map[string]int
}

// Text is the placeholder of a field of Spec.Texts, and empty for any other
// name.
func (f Fields) Text(name string) string {
	index, ok := f.texts[name]
	if !ok {
		return ""
	}
	return f.prefix + "t" + strconv.Itoa(index) + "z"
}

// Address is the placeholder of the address.
func (f Fields) Address() string { return f.prefix + "az" }

var errLimit = errors.New("output limit")

type limited struct {
	out  *bytes.Buffer
	left int
}

func (l *limited) Write(p []byte) (int, error) {
	if len(p) > l.left {
		return 0, errLimit
	}
	l.left -= len(p)
	return l.out.Write(p)
}

// Compile runs source once with the model that the function builds from the
// placeholders, and checks what it writes.
func Compile(source string, spec Spec, model func(Fields) any) (*Fragment, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, refuse(ReasonExecute, err.Error())
	}
	// The prefix and the name of the element that marks a part are not
	// known to the template, so it cannot write either itself.
	nonce := hex.EncodeToString(raw[:])
	fields := Fields{prefix: "flh" + nonce, texts: make(map[string]int, len(spec.Texts))}
	for i, name := range spec.Texts {
		fields.texts[name] = i
	}
	slotElement := "flare-slot-" + nonce

	set, err := template.New("theme").Funcs(spec.Funcs).Parse(source)
	if err != nil {
		return nil, refuse(ReasonParse, err.Error())
	}
	for _, defined := range set.Templates() {
		if strings.HasPrefix(defined.Name(), ReservedPrefix) {
			return nil, refuse(ReasonReservedName, defined.Name())
		}
	}
	slots := make(map[string]Slot, len(spec.Slots))
	for _, slot := range spec.Slots {
		slots[slot.Name] = slot
		if _, err = set.New(slot.Name).Parse("<" + slotElement + ` name="` + slot.Name + `"></` + slotElement + ">"); err != nil {
			return nil, refuse(ReasonExecute, err.Error())
		}
	}
	var out bytes.Buffer
	if err = set.ExecuteTemplate(&limited{out: &out, left: spec.Limit}, "theme", model(fields)); err != nil {
		if errors.Is(err, errLimit) {
			return nil, refuse(ReasonTooLarge, strconv.Itoa(spec.Limit))
		}
		return nil, refuse(ReasonExecute, err.Error())
	}
	p := &parser{
		spec: spec, fields: fields, slots: slots, slotElement: slotElement,
		placeholder: regexp.MustCompile(fields.prefix + `(t[0-9]+|a)z`),
		shown:       make(map[string]int), placed: make(map[string]int), written: make(map[string]bool),
	}
	return p.parse(out.String())
}

type parser struct {
	spec        Spec
	fields      Fields
	slots       map[string]Slot
	slotElement string
	placeholder *regexp.Regexp

	pieces []piece
	static strings.Builder
	stack  []string
	links  int
	// shown counts how often a field stands in the content of an element,
	// and placed how often a part is placed.
	shown, placed map[string]int
	entryLinks    int
	// roots counts the elements that are in no other element, root is the
	// first of them, and loose reports text or a part beside them.
	roots int
	root  string
	loose bool
	// written are the elements the template has written.
	written map[string]bool
}

func (p *parser) hole(h Hole) {
	p.pieces = append(p.pieces, piece{text: p.static.String(), hole: h})
	p.static.Reset()
}

// text writes a value that can hold placeholders of text fields: what is
// between them is escaped, and each of them becomes a hole.
func (p *parser) text(value string, content bool) error {
	if content && len(p.stack) == 0 && strings.TrimSpace(value) != "" {
		p.loose = true
	}
	for {
		at := p.placeholder.FindStringSubmatchIndex(value)
		if at == nil {
			break
		}
		which := value[at[2]:at[3]]
		if which == "a" {
			return refuse(ReasonAddressPosition, "")
		}
		index, err := strconv.Atoi(which[1:])
		if err != nil || index >= len(p.spec.Texts) {
			return refuse(ReasonComputed, "")
		}
		name := p.spec.Texts[index]
		if content {
			p.shown[name]++
		}
		p.static.WriteString(html.EscapeString(value[:at[0]]))
		p.hole(Hole{Kind: HoleText, Name: name})
		value = value[at[1]:]
	}
	p.static.WriteString(html.EscapeString(value))
	return nil
}

func (p *parser) parse(output string) (*Fragment, error) {
	z := html.NewTokenizer(strings.NewReader(output))
	for {
		kind := z.Next()
		if kind == html.ErrorToken {
			if errors.Is(z.Err(), io.EOF) {
				break
			}
			return nil, refuse(ReasonMarkup, z.Err().Error())
		}
		token := z.Token()
		var err error
		switch kind {
		case html.TextToken:
			err = p.text(token.Data, true)
		case html.StartTagToken:
			err = p.start(token)
		case html.SelfClosingTagToken:
			if !void[token.Data] {
				return nil, refuse(ReasonMarkup, "<"+token.Data+"/>")
			}
			err = p.start(token)
		case html.EndTagToken:
			err = p.end(token.Data)
		case html.CommentToken:
			// A comment is not sent.
		default:
			err = refuse(ReasonMarkup, "doctype")
		}
		if err != nil {
			return nil, err
		}
	}
	if len(p.stack) > 0 {
		return nil, refuse(ReasonUnbalanced, p.stack[len(p.stack)-1])
	}
	p.hole(Hole{})
	fragment := &Fragment{pieces: p.pieces}
	for i := range p.pieces {
		// What is left of a placeholder was not placed as it was handed
		// out, and what is left of the mark of a part stood where markup
		// is text. A template that changes a placeholder beyond that
		// writes text of its own: no value of an entry is in it.
		if strings.Contains(p.pieces[i].text, p.fields.prefix) {
			return nil, refuse(ReasonComputed, "")
		}
		if strings.Contains(p.pieces[i].text, p.slotElement) {
			return nil, refuse(ReasonSlotPosition, "")
		}
		fragment.size += len(p.pieces[i].text)
	}
	for _, slot := range p.spec.Slots {
		switch count := p.placed[slot.Name]; {
		case count > 1:
			return nil, refuse(ReasonSlotRepeated, slot.Name)
		case count == 0 && slot.Required:
			return nil, refuse(ReasonSlotMissing, slot.Name)
		}
	}
	if p.spec.Link && p.entryLinks != 1 {
		return nil, refuse(ReasonLinkCount, strconv.Itoa(p.entryLinks))
	}
	for _, name := range p.spec.RequiredTexts {
		if p.shown[name] == 0 {
			return nil, refuse(ReasonTextMissing, name)
		}
	}
	if p.spec.Root != "" && (p.roots != 1 || p.root != p.spec.Root || p.loose) {
		return nil, refuse(ReasonRoot, p.spec.Root)
	}
	for _, name := range p.spec.Elements {
		if !p.written[name] {
			return nil, refuse(ReasonElementMissing, name)
		}
	}
	return fragment, nil
}

func (p *parser) end(name string) error {
	if name == p.slotElement {
		return nil
	}
	if void[name] {
		return refuse(ReasonMarkup, "</"+name+">")
	}
	if len(p.stack) == 0 || p.stack[len(p.stack)-1] != name {
		return refuse(ReasonUnbalanced, name)
	}
	p.stack = p.stack[:len(p.stack)-1]
	if name == "a" {
		p.links--
	}
	p.static.WriteString("</" + name + ">")
	return nil
}

func (p *parser) start(token html.Token) error {
	name := token.Data
	if name == p.slotElement {
		return p.slot(token)
	}
	if !elements[name] {
		return refuse(ReasonElement, name)
	}
	if name == "a" && p.links > 0 {
		return refuse(ReasonMarkup, "a link inside a link")
	}
	p.written[name] = true
	if len(p.stack) == 0 {
		if p.roots == 0 {
			p.root = name
		}
		p.roots++
	}
	// The tokenizer keeps the first of two attributes of one name, as a
	// browser does.
	entry, linked, target := false, false, ""
	for _, attribute := range token.Attr {
		key, value := attribute.Key, attribute.Val
		if strings.Contains(key, p.fields.prefix) {
			return refuse(ReasonFieldPosition, "attribute")
		}
		if !allowedAttribute(name, key) {
			return refuse(ReasonAttribute, key)
		}
		holds := p.placeholder.MatchString(value)
		switch {
		case key == "href":
			linked = true
			if value == p.fields.Address() && p.spec.Link {
				entry = true
			} else if holds {
				return refuse(ReasonAddressPosition, "")
			} else if !staticLink(value) {
				return refuse(ReasonLink, value)
			}
		case key == "src" || key == "srcset":
			if holds {
				return refuse(ReasonFieldPosition, key)
			}
			if !p.assets(key, value) {
				return refuse(ReasonResource, value)
			}
		case key == "target":
			target = value
		case holds && !textAttribute(key):
			return refuse(ReasonFieldPosition, key)
		}
	}
	// The link to the address gets its target from the application. A link
	// of the template can open a new window and cannot name another one.
	if !entry && target != "" && target != "_blank" && target != "_self" {
		return refuse(ReasonAttribute, "target")
	}
	// The tag is written out again, so that how it is read does not depend
	// on how the template spelled it.
	p.static.WriteString("<" + name)
	for _, attribute := range token.Attr {
		if name == "a" && (attribute.Key == "rel" || attribute.Key == "target") {
			continue
		}
		p.static.WriteString(" " + attribute.Key + `="`)
		if entry && attribute.Key == "href" {
			p.hole(Hole{Kind: HoleAddress})
		} else if err := p.text(attribute.Val, false); err != nil {
			return err
		}
		p.static.WriteString(`"`)
	}
	switch {
	case entry:
		p.entryLinks++
		p.hole(Hole{Kind: HoleLink})
	case linked:
		// A link of the template itself tells the page it leads to
		// nothing about the instance.
		p.static.WriteString(` rel="noopener noreferrer"`)
		if target == "_blank" {
			p.static.WriteString(` target="_blank"`)
		}
	}
	p.static.WriteString(">")
	if !void[name] {
		p.stack = append(p.stack, name)
		if name == "a" {
			p.links++
		}
	}
	return nil
}

func (p *parser) slot(token html.Token) error {
	name := ""
	for _, attribute := range token.Attr {
		if attribute.Key == "name" {
			name = attribute.Val
		}
	}
	slot, known := p.slots[name]
	if !known {
		return refuse(ReasonSlotPosition, name)
	}
	if p.links > 0 && !slot.Inline {
		return refuse(ReasonSlotPosition, name)
	}
	if len(p.stack) == 0 {
		p.loose = true
	}
	if len(slot.Within) > 0 && (len(p.stack) == 0 || !slices.Contains(slot.Within, p.stack[len(p.stack)-1])) {
		return refuse(ReasonSlotPosition, name)
	}
	p.placed[name]++
	p.hole(Hole{Kind: HoleSlot, Name: name})
	return nil
}

var assetName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// assets reports whether the value of src, or every address of srcset, is a
// file among the assets of the theme.
func (p *parser) assets(key, value string) bool {
	asset := func(address string) bool {
		rest, ok := strings.CutPrefix(address, p.spec.AssetPrefix)
		return ok && p.spec.AssetPrefix != "" && assetName.MatchString(rest) && !strings.Contains(rest, "..")
	}
	if key == "src" {
		return asset(value)
	}
	for _, candidate := range strings.Split(value, ",") {
		parts := strings.Fields(candidate)
		if len(parts) == 0 || len(parts) > 2 || !asset(parts[0]) {
			return false
		}
	}
	return true
}

// staticLink reports whether a link written in a template leads to a page:
// an address without a scheme, or one with http, https or mailto. What
// stands before the first colon is compared as it is, so a scheme spelled
// with a tab or another character in it is none of the three.
func staticLink(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if strings.HasPrefix(lower, "//") {
		return true
	}
	if i := strings.IndexAny(lower, ":/?#"); i >= 0 && lower[i] == ':' {
		scheme := lower[:i]
		return scheme == "http" || scheme == "https" || scheme == "mailto"
	}
	return true
}

func textAttribute(key string) bool {
	return key == "title" || key == "alt" || key == "aria-label" || strings.HasPrefix(key, "data-")
}

var void = map[string]bool{"br": true, "col": true, "hr": true, "img": true, "source": true, "wbr": true}

// elements are those a template can hold. Elements that run or load
// something, that are read in a way of their own, or that the application
// relies on for its own parts are not among them: script, style, iframe,
// object, embed, base, link, meta, title, form and its controls, svg, math,
// template and noscript.
var elements = map[string]bool{
	"a": true, "abbr": true, "address": true, "article": true, "aside": true, "b": true, "blockquote": true,
	"br": true, "caption": true, "cite": true, "code": true, "col": true, "colgroup": true, "dd": true, "del": true,
	"details": true, "dfn": true, "div": true, "dl": true, "dt": true, "em": true, "figcaption": true, "figure": true,
	"footer": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "header": true,
	"hgroup": true, "hr": true, "i": true, "img": true, "ins": true, "kbd": true, "li": true, "main": true,
	"mark": true, "nav": true, "ol": true, "p": true, "picture": true, "pre": true, "q": true, "s": true,
	"samp": true, "section": true, "small": true, "source": true, "span": true, "strong": true, "sub": true,
	"summary": true, "sup": true, "table": true, "tbody": true, "td": true, "tfoot": true, "th": true,
	"thead": true, "time": true, "tr": true, "u": true, "ul": true, "var": true, "wbr": true,
}

var globalAttributes = map[string]bool{
	"class": true, "title": true, "lang": true, "dir": true, "hidden": true, "role": true, "tabindex": true, "translate": true,
}

var elementAttributes = map[string]map[string]bool{
	"a":        {"href": true, "hreflang": true, "rel": true, "target": true},
	"img":      {"src": true, "srcset": true, "sizes": true, "alt": true, "width": true, "height": true, "loading": true, "decoding": true},
	"source":   {"srcset": true, "sizes": true, "media": true, "type": true},
	"details":  {"open": true},
	"time":     {"datetime": true},
	"ol":       {"start": true, "reversed": true, "type": true},
	"li":       {"value": true},
	"td":       {"colspan": true, "rowspan": true, "headers": true},
	"th":       {"colspan": true, "rowspan": true, "headers": true, "scope": true},
	"col":      {"span": true},
	"colgroup": {"span": true},
}

var attributeName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// allowedAttribute reports whether an element can have the attribute. There
// is no id and no name, which scripts of the application look elements up
// by, no style, which belongs in the stylesheet of the theme, and no
// attribute that handles an event or sends anything.
func allowedAttribute(element, key string) bool {
	if !attributeName.MatchString(key) {
		return false
	}
	if globalAttributes[key] || elementAttributes[element][key] {
		return true
	}
	return (strings.HasPrefix(key, "aria-") && len(key) > len("aria-")) || (strings.HasPrefix(key, "data-") && len(key) > len("data-"))
}
