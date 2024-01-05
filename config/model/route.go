package FlareModel

// Page Data Model
type Page struct {
	Title string `json:"Title"`
	Name  string `json:"Name"`
	Path  string `json:"Path"`
	Alias string `json:"Alias"`
}

// API Data Model
type API struct {
	Name  string `json:"Name"`
	Path  string `json:"Path"`
	Alias string `json:"Alias"`
}

// Route Map for Web Server
type RouteMaps struct {
	// Pages
	Home         Page `json:"Home"`
	Applications Page `json:"Applications"`
	Bookmarks    Page `json:"Bookmarks"`
	Help         Page `json:"Help"`
	Guide        Page `json:"Guide"`
	Editor       Page `json:"Editor"`
	Icons        Page `json:"Icons"`
	Preview      Page `json:"Preview"`

	// Settings Pages
	Settings   Page `json:"Settings"`
	Theme      Page `json:"Theme"`
	Weather    Page `json:"Weather"`
	Search     Page `json:"Search"`
	Appearance Page `json:"Appearance"`
	Others     Page `json:"Others"`

	// Auth API
	Login  API `json:"Login"`
	Logout API `json:"Logout"`

	// Misc
	RedirHome   Page `json:"RedirHome"`
	RedirHelper API  `json:"RedirHelper"`
	WeatherTest API  `json:"WeatherTest"`
	HealthCheck API  `json:"HealthCheck"`
}
