package model

// Application Data Model
type Application struct {
	Title                   string `yaml:"Title"`
	Footer                  string `yaml:"Footer"`
	OpenAppNewTab           bool   `yaml:"OpenAppNewTab"`
	OpenBookmarkNewTab      bool   `yaml:"OpenBookmarkNewTab"`
	ShowTitle               bool   `yaml:"ShowTitle"`
	Greetings               string `yaml:"Greetings"`
	ShowSearchComponent     bool   `yaml:"ShowSearchComponent"`
	DisabledSearchAutoFocus bool   `yaml:"DisabledSearchAutoFocus"`
	ShowDateTime            bool   `yaml:"ShowDateTime"`
	ShowApps                bool   `yaml:"ShowApps"`
	ShowBookmarks           bool   `yaml:"ShowBookmarks"`
	Theme                   string `yaml:"Theme"`
	ShowWeather             bool   `yaml:"ShowWeather"`
	Location                string `yaml:"Location"`
}
