package model

// Weather Data Model
type Weather struct {
	ExternalLastUpdate string `json:"externalLastUpdate"`
	Degree             int    `json:"degree"`
	IsDay              bool   `json:"isDay"`
	ConditionText      string `json:"conditionText"`
	ConditionCode      string `json:"conditionCode"`
	Humidity           int    `json:"humidity"`
	Expires            int64  `json:"expires"`
	Location           string `json:"location"`
}

// WeatherLocation preserves the provider's exact names and candidate identity.
type WeatherLocation struct {
	ID       string `yaml:"ID" json:"id"`
	Province string `yaml:"Province" json:"province"`
	City     string `yaml:"City" json:"city"`
	County   string `yaml:"County" json:"county"`
}

func (l WeatherLocation) String() string {
	value := l.Province + ", " + l.City
	if l.County != "" {
		value += ", " + l.County
	}
	return value
}
