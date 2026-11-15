package model

import (
	"strings"
	"unicode/utf8"
)

// A theme that is a directory is handed out and taken in as an archive: a ZIP
// file with the files of one theme. They are the theme file, the stylesheet,
// the images and fonts of the assets, and text documents such as a license,
// which travel with a theme without being read or served.
const (
	// ThemeManifestName is the theme file of a theme that is a directory, and
	// ThemeStylesheetName the stylesheet such a theme can bring.
	ThemeManifestName   = "theme.yml"
	ThemeStylesheetName = "theme.css"
	// MaximumThemeArchiveBytes is the largest archive of a theme.
	MaximumThemeArchiveBytes = 24 << 20
	// MaximumThemeArchiveEntries is the number of entries an archive can
	// have. One with more is not looked into.
	MaximumThemeArchiveEntries = 256
	// MaximumThemeDocuments is the number of text documents that travel with
	// a theme, and MaximumThemeDocumentBytes the largest of them.
	MaximumThemeDocuments     = 8
	MaximumThemeDocumentBytes = 64 << 10
)

// ThemeArchiveKind says what an entry of a theme archive is.
type ThemeArchiveKind int

const (
	// ThemeArchiveOther is no file of a theme. It is not stored, and it is
	// named to whoever imports the archive.
	ThemeArchiveOther ThemeArchiveKind = iota
	// ThemeArchiveIgnored is a hidden entry or one a system puts into an
	// archive by itself. It is not stored, and nothing is said about it.
	ThemeArchiveIgnored
	// ThemeArchiveManifest is the theme file.
	ThemeArchiveManifest
	// ThemeArchiveStylesheet is the stylesheet.
	ThemeArchiveStylesheet
	// ThemeArchiveAsset is a file of the assets, by its extension. Whether
	// its name and its content are accepted is for the reader of the theme.
	ThemeArchiveAsset
	// ThemeArchiveDocument is a text document beside the theme file.
	ThemeArchiveDocument
)

// themeDocumentNames are the names of text documents without an extension.
var themeDocumentNames = []string{"LICENSE", "NOTICE", "COPYING", "README"}

// IsThemeDocumentName reports whether a name is that of a text document that
// travels with a theme: LICENSE, NOTICE, COPYING or README, or a name that
// ends in .txt or .md in any case. The name has the form of a name among the
// assets, which is the same in a file system and in an archive and names no
// other directory.
func IsThemeDocumentName(name string) bool {
	if !IsThemeAssetName(name) {
		return false
	}
	for _, document := range themeDocumentNames {
		if name == document {
			return true
		}
	}
	lower := strings.ToLower(name)
	// A name that is nothing but an extension begins with a dot, and is no
	// name among the assets.
	for _, extension := range []string{".txt", ".md"} {
		if strings.HasSuffix(lower, extension) {
			return true
		}
	}
	return false
}

// IsThemeDocumentContent reports whether the content of a file is that of a
// text document: UTF-8 text of a bounded size without a NUL.
func IsThemeDocumentContent(content []byte) bool {
	return len(content) <= MaximumThemeDocumentBytes && utf8.Valid(content) && !strings.ContainsRune(string(content), 0)
}

// ThemeArchiveEntry says what an entry of a theme archive is, by its path in
// the theme with slashes between its parts, and returns the name of the file
// for an asset and for a document. The path is taken as it is written: a
// path that is not one of the forms below is no file of a theme, whatever it
// would name once it were cleaned.
func ThemeArchiveEntry(path string) (ThemeArchiveKind, string) {
	parts := strings.Split(path, "/")
	for _, part := range parts {
		// Hidden entries are what editors and file managers leave behind,
		// and __MACOSX is what an archive made on a Mac holds beside its
		// files.
		if (strings.HasPrefix(part, ".") && part != "." && part != "..") || part == "__MACOSX" {
			return ThemeArchiveIgnored, ""
		}
	}
	switch {
	case len(parts) == 1 && parts[0] == ThemeManifestName:
		return ThemeArchiveManifest, ""
	case len(parts) == 1 && parts[0] == ThemeStylesheetName:
		return ThemeArchiveStylesheet, ""
	case len(parts) == 1 && IsThemeDocumentName(parts[0]):
		return ThemeArchiveDocument, parts[0]
	case len(parts) == 2 && parts[0] == ThemeAssetDirectory && HasThemeAssetExtension(parts[1]):
		return ThemeArchiveAsset, parts[1]
	}
	return ThemeArchiveOther, ""
}
