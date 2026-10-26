package model

// SearchOptions configures the homepage's search targets. A missing block keeps legacy local search.
type SearchOptions struct {
	// Mode is hybrid (live local matches + web submit), local-first, or select.
	Mode          string         `yaml:"Mode"`
	DefaultTarget string         `yaml:"DefaultTarget"`
	OpenNewTab    bool           `yaml:"OpenNewTab"`
	Engines       []SearchEngine `yaml:"Engines"`
}

// SearchEngine is a GET search template administered by the instance owner.
type SearchEngine struct {
	ID          string `yaml:"ID"`
	Name        string `yaml:"Name"`
	URLTemplate string `yaml:"URLTemplate"`
	Enabled     bool   `yaml:"Enabled"`
}
