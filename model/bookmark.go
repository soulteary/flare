package model

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

// Generic Ordinary Bookmarks Data Model
type OrdinaryBookmarks struct {
	Categories []Category `yaml:"categories"`
	Items      []Bookmark `yaml:"links"`
}

// Generic Favorite Bookmarks Data Model
type FavoriteBookmarks struct {
	Items []Bookmark `yaml:"links"`
}
