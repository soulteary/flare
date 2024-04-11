package FlareModel

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
	DisableCSP             bool

	Visibility       string
	DisableLoginMode bool
	User             string
	Pass             string
	UserIsGenerated  bool
	PassIsGenerated  bool

	CookieName   string
	CookieSecret string
}

// Application Envs Data Model
type Envs struct {
	Port                   int  `env:"FLARE_PORT"`
	EnableGuide            bool `env:"FLARE_GUIDE"`
	EnableEditor           bool `env:"FLARE_EDITOR"`
	EnableOfflineMode      bool `env:"FLARE_OFFLINE"`
	EnableMinimumRequest   bool `env:"FLARE_MINI_REQUEST"`
	EnableDeprecatedNotice bool `env:"FLARE_DEPRECATED_NOTICE"`
	DisableCSP             bool `env:"FLARE_DISABLE_CSP,unset"`

	Visibility       string `env:"FLARE_VISIBILITY"`
	DisableLoginMode bool   `env:"FLARE_DISABLE_LOGIN"`
	User             string `env:"FLARE_USER,unset"`
	Pass             string `env:"FLARE_PASS,unset"`

	CookieName   string `env:"FLARE_COOKIE_NAME"`
	CookieSecret string `env:"FLARE_COOKIE_SECRET"`
}

// Application Envfile Data Model
type EnvFile struct {
	Port                   int  `ini:"FLARE_PORT,omitempty"`
	EnableGuide            bool `ini:"FLARE_GUIDE,omitempty"`
	EnableEditor           bool `ini:"FLARE_EDITOR,omitempty"`
	EnableOfflineMode      bool `ini:"FLARE_OFFLINE,omitempty"`
	EnableMinimumRequest   bool `ini:"FLARE_MINI_REQUEST,omitempty"`
	EnableDeprecatedNotice bool `ini:"FLARE_DEPRECATED_NOTICE,omitempty"`
	DisableCSP             bool `env:"FLARE_DISABLE_CSP,unset"`

	Visibility       string `ini:"FLARE_VISIBILITY,omitempty"`
	DisableLoginMode bool   `ini:"FLARE_DISABLE_LOGIN,omitempty"`
	User             string `ini:"FLARE_USER,omitempty"`
	Pass             string `ini:"FLARE_PASS,omitempty"`

	CookieName   string `ini:"FLARE_COOKIE_NAME,omitempty"`
	CookieSecret string `ini:"FLARE_COOKIE_SECRET,omitempty"`
}
