package themetemplate

// Reason says why a template of a theme was not compiled.
type Reason string

const (
	// ReasonParse: the source is not a template.
	ReasonParse Reason = "parse"
	// ReasonReservedName: the source defines a template under a name that
	// begins with flare/, which names the parts the application renders.
	ReasonReservedName Reason = "reserved_name"
	// ReasonExecute: running the template failed, for example because it
	// calls a template that does not exist or places a part where markup
	// cannot stand.
	ReasonExecute Reason = "execute"
	// ReasonTooLarge: the template writes more than it may.
	ReasonTooLarge Reason = "too_large"
	// ReasonMarkup: the output holds markup that is not read, such as a
	// doctype, a link inside a link or an element closed where it has no end.
	ReasonMarkup Reason = "markup"
	// ReasonElement: an element that a template cannot hold.
	ReasonElement Reason = "element"
	// ReasonAttribute: an attribute, or a value of one, that a template
	// cannot hold.
	ReasonAttribute Reason = "attribute"
	// ReasonUnbalanced: an element is not closed where it was opened.
	ReasonUnbalanced Reason = "unbalanced"
	// ReasonFieldPosition: a field stands where it cannot be filled in
	// safely, such as a class or the address of an image.
	ReasonFieldPosition Reason = "field_position"
	// ReasonAddressPosition: the address stands anywhere but as the whole
	// href of a link.
	ReasonAddressPosition Reason = "address_position"
	// ReasonComputed: the template did something with a field other than
	// placing it.
	ReasonComputed Reason = "computed"
	// ReasonLink: a link written in the template has an address of a kind
	// that is not accepted.
	ReasonLink Reason = "link"
	// ReasonResource: an address that loads something is not a file among
	// the assets of the theme.
	ReasonResource Reason = "resource"
	// ReasonSlotPosition: a part stands where it cannot, such as inside a
	// link.
	ReasonSlotPosition Reason = "slot_position"
	// ReasonSlotMissing: a part that has to be placed is not.
	ReasonSlotMissing Reason = "slot_missing"
	// ReasonSlotRepeated: a part is placed more than once.
	ReasonSlotRepeated Reason = "slot_repeated"
	// ReasonLinkCount: the template does not have exactly one link to the
	// address.
	ReasonLinkCount Reason = "link_count"
	// ReasonTextMissing: a field that has to be shown as text is not.
	ReasonTextMissing Reason = "text_missing"
)

// Error is why a template was not compiled. Subject is what the reason is
// about: an element, an attribute, a part or a field, or what the template
// engine said.
type Error struct {
	Reason  Reason
	Subject string
}

func (e *Error) Error() string {
	if e.Subject == "" {
		return "theme template: " + string(e.Reason)
	}
	return "theme template: " + string(e.Reason) + ": " + e.Subject
}

func refuse(reason Reason, subject string) error { return &Error{Reason: reason, Subject: subject} }
