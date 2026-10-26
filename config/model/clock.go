package model

// ClockOptions is optional so old configuration files retain their exact behavior.
// Seconds is a pointer to distinguish an omitted value (true) from explicit false.
type ClockOptions struct {
	Dynamic      bool   `yaml:"Dynamic"`
	Seconds      *bool  `yaml:"Seconds"`
	TimezoneMode string `yaml:"TimezoneMode"`
	Timezone     string `yaml:"Timezone"`
}

func (c ClockOptions) ShowSeconds() bool { return c.Seconds == nil || *c.Seconds }
