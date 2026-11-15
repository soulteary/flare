package themetemplate

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"regexp"
)

// Builtin compiles a template of the application itself into a fragment
// whose holes are the named parts. The pages of the application are put
// together the way a page template of a theme is: its parts are rendered and
// filled into the holes.
//
// The template is trusted. What it writes is taken as it is and not checked,
// so a fragment renders the bytes the template would have written with its
// parts in place. Each part has to be placed once.
//
// The parts are defined anew in set, which is therefore a clone that is used
// for nothing else and has not been executed.
func Builtin(set *template.Template, name string, slots []string, data any) (*Fragment, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, refuse(ReasonExecute, err.Error())
	}
	element := "flare-slot-" + hex.EncodeToString(raw[:])
	for _, slot := range slots {
		if _, err := set.New(slot).Parse("<" + element + ` name="` + slot + `"></` + element + ">"); err != nil {
			return nil, refuse(ReasonExecute, err.Error())
		}
	}
	var out bytes.Buffer
	if err := set.ExecuteTemplate(&out, name, data); err != nil {
		return nil, refuse(ReasonExecute, err.Error())
	}
	output := out.String()
	mark := regexp.MustCompile("<" + element + ` name="([^"]*)"></` + element + ">")
	fragment := &Fragment{}
	placed := make(map[string]int, len(slots))
	at := 0
	for _, found := range mark.FindAllStringSubmatchIndex(output, -1) {
		slot := output[found[2]:found[3]]
		placed[slot]++
		fragment.pieces = append(fragment.pieces, piece{text: output[at:found[0]], hole: Hole{Kind: HoleSlot, Name: slot}})
		fragment.size += found[0] - at
		at = found[1]
	}
	fragment.pieces = append(fragment.pieces, piece{text: output[at:]})
	fragment.size += len(output) - at
	for _, slot := range slots {
		switch count := placed[slot]; {
		case count == 0:
			return nil, refuse(ReasonSlotMissing, slot)
		case count > 1:
			return nil, refuse(ReasonSlotRepeated, slot)
		}
	}
	return fragment, nil
}
