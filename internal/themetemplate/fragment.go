package themetemplate

import (
	"bytes"
	"html/template"
)

// HoleKind is what a hole of a fragment is filled with.
type HoleKind int

const (
	// HoleText is a field shown as text, in the content of an element or
	// in the value of an attribute. It is escaped when it is filled in.
	HoleText HoleKind = iota + 1
	// HoleAddress is the address, as the whole value of the href of a link.
	HoleAddress
	// HoleLink stands at the end of the attributes of the link to the
	// address, where the application writes rel and target.
	HoleLink
	// HoleSlot is a part that the application renders.
	HoleSlot
)

// Hole is one place of a fragment that is filled in for each rendering.
type Hole struct {
	Kind HoleKind
	// Name is the field of a text and the name of a part. It is empty for
	// the address and for the attributes of its link.
	Name string
}

type piece struct {
	text string
	hole Hole
}

// Fragment is a compiled template: text that does not change, and holes.
// Rendering one runs no template.
type Fragment struct {
	pieces []piece
	size   int
}

// Content is what the holes of a fragment are filled with.
type Content interface {
	// Text is the value of a field as it is. Render escapes it.
	Text(field string) string
	// Address is the address of the link, checked by the application.
	// Render escapes it for the attribute.
	Address() string
	// LinkAttributes is written after the other attributes of the link to
	// the address. It begins with a space or is empty.
	LinkAttributes() template.HTMLAttr
	// Slot is the markup of a part, as the application rendered it.
	Slot(name string) template.HTML
}

// SlotWriter is a Content that writes a part itself, where it belongs,
// instead of handing its markup over. Render uses it in place of Slot.
type SlotWriter interface {
	WriteSlot(out *bytes.Buffer, name string)
}

// Render writes the fragment with its holes filled in.
func (f *Fragment) Render(out *bytes.Buffer, content Content) {
	writer, direct := content.(SlotWriter)
	for i := range f.pieces {
		out.WriteString(f.pieces[i].text)
		switch f.pieces[i].hole.Kind {
		case HoleText:
			template.HTMLEscape(out, []byte(content.Text(f.pieces[i].hole.Name)))
		case HoleAddress:
			template.HTMLEscape(out, []byte(content.Address()))
		case HoleLink:
			out.WriteString(string(content.LinkAttributes()))
		case HoleSlot:
			if direct {
				writer.WriteSlot(out, f.pieces[i].hole.Name)
			} else {
				out.WriteString(string(content.Slot(f.pieces[i].hole.Name)))
			}
		}
	}
}

// Holes lists the holes of the fragment in the order they stand in.
func (f *Fragment) Holes() []Hole {
	var holes []Hole
	for i := range f.pieces {
		if f.pieces[i].hole.Kind != 0 {
			holes = append(holes, f.pieces[i].hole)
		}
	}
	return holes
}

// Size is the number of bytes of the text between the holes.
func (f *Fragment) Size() int { return f.size }
