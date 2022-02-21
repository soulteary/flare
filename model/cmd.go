package model

// Application Flags Data Model
type Flags struct {
	DebugMode   bool
	ShowVersion bool
	ShowHelp    bool

	Port                   int
	EnableGuide            bool
	EnableEditor           bool
	EnableOfflineMode      bool
	EnableMinimumRequest   bool
	EnableDeprecatedNotice bool

	DisableLoginMode     bool
	FlareUser            string
	FlareUserIsGenerated bool
	FlarePass            string
	FlarePassIsGenerated bool
}

// Application Envs Data Model
type Envs struct {
	Port                   int  `env:"FLARE_PORT"`
	EnableGuide            bool `env:"FLARE_GUIDE"`
	EnableEditor           bool `env:"FLARE_EDITOR"`
	EnableOfflineMode      bool `env:"FLARE_OFFLINE"`
	EnableMinimumRequest   bool `env:"FLARE_MINI_REQUEST"`
	EnableDeprecatedNotice bool `env:"FLARE_DEPRECATED_NOTICE"`

	DisableLoginMode bool   `env:"FLARE_DISABLE_LOGIN"`
	FlareUser        string `env:"FLARE_USER,unset"`
	FlarePass        string `env:"FLARE_PASS,unset"`
}
