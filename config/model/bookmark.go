package FlareModel

// Generic Bookmark Data Model
type Bookmark struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"link"`
	Icon     string `yaml:"icon,omitempty"`
	Desc     string `yaml:"desc,omitempty"`
	Private  bool   `yaml:"private,omitempty"`
	Category string `yaml:"category,omitempty"`
}

// Generic Category Data Model
type Category struct {
	ID   string `yaml:"id"`
	Name string `yaml:"title"`
}

// Generic Bookmarks Data Model
type Bookmarks struct {
	Categories []Category `yaml:"categories,omitempty"`
	Items      []Bookmark `yaml:"links"`
}
