// Package bookmarks parses and plans bookmark transfers without network access or disk writes.
package bookmarks

import "github.com/soulteary/flare/config/model"

// Transfer limits bound parsing and preview memory usage.
const (
	MaxBytes = 8 << 20
	MaxItems = 20000
	MaxDepth = 32
)

// Entry is a source bookmark and its directory path.
type Entry struct {
	Bookmark model.Bookmark
	Path     []string
}

// Options selects target mapping and duplicate behavior.
type Options struct {
	Strategy   string `json:"strategy"`
	Folder     string `json:"folder"`
	Flatten    bool   `json:"flatten"`
	StripRoot  bool   `json:"stripRoot"`
	AllowOther bool   `json:"allowOther"`
	Apps       bool   `json:"-"`
}

// Row describes the proposed outcome for one source bookmark.
type Row struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Name   string `json:"name"`
	URL    string `json:"url"`
	Action string `json:"action"`
	Reason string `json:"reason"`
}

// Plan contains the preview and its resulting collection.
type Plan struct {
	Rows       []Row           `json:"rows"`
	Added      int             `json:"added"`
	Updated    int             `json:"updated"`
	Skipped    int             `json:"skipped"`
	Invalid    int             `json:"invalid"`
	Categories int             `json:"categories"`
	Result     model.Bookmarks `json:"-"`
}
