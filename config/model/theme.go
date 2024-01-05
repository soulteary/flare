package FlareModel

// Theme color Data Model
type Palette struct {
	Background string `json:"background"`
	Primary    string `json:"primary"`
	Accent     string `json:"accent"`
}

// Theme Data Model
type Theme struct {
	Name   string  `json:"name"`
	Colors Palette `json:"colors"`
}
