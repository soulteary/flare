package model

import (
	"bytes"
	"encoding/binary"
	"strings"
	"time"
)

// A theme that is a directory can bring images and fonts in a directory named
// assets beside its stylesheet. The stylesheet refers to them as
// url(assets/<name>).
const (
	// ThemeAssetDirectory is the directory of a theme that holds its assets.
	ThemeAssetDirectory = "assets"
	// MaximumThemeAssets is the number of files the assets of a theme can hold.
	MaximumThemeAssets = 32
	// MaximumThemeAssetBytes is the largest file among the assets.
	MaximumThemeAssetBytes = 2 << 20
	// maximumThemeAssetName is the longest name of a file among the assets.
	maximumThemeAssetName = 64
)

// ThemeAsset is a file a theme brings in its assets: its name, the type it is
// served as, and its state when the themes directory was read.
type ThemeAsset struct {
	Name    string
	Type    string
	Size    int64
	ModTime time.Time
}

// themeAssetKind is a kind of file the assets of a theme can hold: the type
// it is served as, and how its content starts.
type themeAssetKind struct {
	contentType string
	starts      func(head []byte) bool
}

func startsWith(prefixes ...string) func([]byte) bool {
	return func(head []byte) bool {
		for _, prefix := range prefixes {
			if bytes.HasPrefix(head, []byte(prefix)) {
				return true
			}
		}
		return false
	}
}

// startsAsAVIF reports the start of an AVIF image: a file type box that names
// avif or avis as its brand or among the brands it is compatible with, as far
// as the start of the file holds them. An encoder is free to name a more
// general brand first.
func startsAsAVIF(head []byte) bool {
	if len(head) < 12 || string(head[4:8]) != "ftyp" {
		return false
	}
	// The box says how long it is. After its name come a brand, a version,
	// and the brands the file is compatible with, of four bytes each.
	end := len(head)
	if size := binary.BigEndian.Uint32(head); uint64(size) < uint64(end) {
		end = int(size)
	}
	for at := 8; at+4 <= end; at += 4 {
		if brand := string(head[at : at+4]); at != 12 && (brand == "avif" || brand == "avis") {
			return true
		}
	}
	return false
}

// startsAsFont reports the start of a font in the container TrueType and
// OpenType fonts share. Which outlines a file holds does not follow from the
// extension of its name: both extensions are in use for both.
var startsAsFont = startsWith("\x00\x01\x00\x00", "OTTO", "true")

// themeAssetKinds lists the files a theme can bring, by the extension of
// their names in lower case: images that are pictures and nothing else, and
// fonts. A format that can carry scripts or refer to other files, as SVG can,
// is not among them.
var themeAssetKinds = map[string]themeAssetKind{
	"png":  {"image/png", startsWith("\x89PNG\r\n\x1a\n")},
	"jpg":  {"image/jpeg", startsWith("\xff\xd8\xff")},
	"jpeg": {"image/jpeg", startsWith("\xff\xd8\xff")},
	"gif":  {"image/gif", startsWith("GIF87a", "GIF89a")},
	"webp": {"image/webp", func(head []byte) bool {
		return len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP"
	}},
	"avif":  {"image/avif", startsAsAVIF},
	"woff2": {"font/woff2", startsWith("wOF2")},
	"woff":  {"font/woff", startsWith("wOFF")},
	"ttf":   {"font/ttf", startsAsFont},
	"otf":   {"font/otf", startsAsFont},
}

// ThemeAssetHeadBytes is how much of a file IsThemeAssetContent looks at.
const ThemeAssetHeadBytes = 64

// IsThemeAssetName reports whether a name can be that of a file among the
// assets of a theme: letters, digits, dots, hyphens and underscores of ASCII,
// starting with a letter or a digit, of a bounded length. Such a name is the
// same in a file system and in an address, and names no other directory.
func IsThemeAssetName(name string) bool {
	if name == "" || len(name) > maximumThemeAssetName {
		return false
	}
	for i, c := range []byte(name) {
		letter := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !letter && (i == 0 || (c != '.' && c != '-' && c != '_')) {
			return false
		}
	}
	return true
}

// HasThemeAssetExtension reports whether a name ends in the extension of a
// kind of file a theme can bring, in any case. Such a file in the assets of a
// theme is meant as one of them, whatever else its name holds.
func HasThemeAssetExtension(name string) bool {
	dot := strings.LastIndexByte(name, '.')
	return dot >= 0 && themeAssetKinds[asciiLower(name[dot+1:])].starts != nil
}

// themeAssetKindOf returns the kind of a file among the assets, by the
// extension of its name in any case. It reports false for a name a file among
// the assets cannot have, and for an extension that is not listed.
func themeAssetKindOf(name string) (themeAssetKind, bool) {
	if !IsThemeAssetName(name) || !HasThemeAssetExtension(name) {
		return themeAssetKind{}, false
	}
	return themeAssetKinds[asciiLower(name[strings.LastIndexByte(name, '.')+1:])], true
}

// ThemeAssetType returns the type a file among the assets is served as, by
// the extension of its name. It reports false for a file a theme cannot bring.
func ThemeAssetType(name string) (string, bool) {
	kind, known := themeAssetKindOf(name)
	return kind.contentType, known
}

// IsThemeAssetContent reports whether the start of a file is that of the kind
// of file its name says it is.
func IsThemeAssetContent(name string, head []byte) bool {
	kind, known := themeAssetKindOf(name)
	return known && kind.starts(head)
}

// ThemeAssetReference reads an address of a stylesheet as a reference to a
// file among the assets of its theme: assets/<name>, and nothing before,
// between or after. It returns the name of the file. An address of any other
// form, with a scheme, a host, a query, a fragment, another directory or a
// character a name cannot have, is not such a reference.
func ThemeAssetReference(address string) (string, bool) {
	name, inAssets := strings.CutPrefix(address, ThemeAssetDirectory+"/")
	if !inAssets || !IsThemeAssetName(name) {
		return "", false
	}
	return name, true
}
