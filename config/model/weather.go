package FlareModel

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
