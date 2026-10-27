package model

// Application Data Model
type Application struct {
	ConfigRevision          string           `yaml:"-"`
	AppVersion              string           `yaml:"AppVersion,omitempty"`
	ConfigVersionState      string           `yaml:"-"`
	Search                  *SearchOptions   `yaml:"Search,omitempty"`
	Homepage                *HomepageOptions `yaml:"Homepage,omitempty"`
	Clock                   *ClockOptions    `yaml:"Clock,omitempty"`
	Title                   string           `yaml:"Title"`
	Footer                  string           `yaml:"Footer"`
	LoginFooter             string           `yaml:"LoginFooter,omitempty"`
	OpenAppNewTab           bool             `yaml:"OpenAppNewTab"`
	OpenBookmarkNewTab      bool             `yaml:"OpenBookmarkNewTab"`
	ShowTitle               bool             `yaml:"ShowTitle"`
	Greetings               string           `yaml:"Greetings"`
	ShowSearchComponent     bool             `yaml:"ShowSearchComponent"`
	DisabledSearchAutoFocus bool             `yaml:"DisabledSearchAutoFocus"`
	ShowDateTime            bool             `yaml:"ShowDateTime"`
	ShowApps                bool             `yaml:"ShowApps"`
	ShowBookmarks           bool             `yaml:"ShowBookmarks"`
	HideSettingsButton      bool             `yaml:"HideSettingButton"`
	HideVersion             bool             `yaml:"HideVersion"`
	HideHelpButton          bool             `yaml:"HideHelpButton"`
	Theme                   string           `yaml:"Theme"`
	ShowWeather             bool             `yaml:"ShowWeather"`
	WeatherLocation         *WeatherLocation `yaml:"WeatherLocation,omitempty"`
	Location                string           `yaml:"Location"`
	EnableEncryptedLink     bool             `yaml:"EnableEncryptedLink"`
	IconMode                string           `yaml:"IconMode"`
	KeepLetterCase          bool             `yaml:"KeepLetterCase"`
	Locale                  string           `yaml:"Locale"`
}
