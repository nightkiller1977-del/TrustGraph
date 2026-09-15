package config

type Config struct {
	Port        string `envconfig:"PORT" default:"8080"`
	Environment string `envconfig:"ENVIRONMENT" default:"development"`

	DatabaseURL string `envconfig:"DATABASE_URL" default:""`

	AssessmentTimeoutMS int `envconfig:"ASSESSMENT_TIMEOUT_MS" default:"300"`

	CircuitBreakerMaxFailures int `envconfig:"CIRCUIT_BREAKER_MAX_FAILURES" default:"5"`
	CircuitBreakerWindowMins  int `envconfig:"CIRCUIT_BREAKER_WINDOW_MINS" default:"5"`

	IdempotencyTTLHours int `envconfig:"IDEMPOTENCY_TTL_HOURS" default:"24"`

	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`

	// Phase 1.5: shadow mode — run assessments without enforcing capability gates
	EnforcementEnabled bool   `envconfig:"ENFORCEMENT_ENABLED" default:"false"`
	AdminToken         string `envconfig:"ADMIN_TOKEN" default:""`

	// EnforcementThreshold is the risk-score cutoff (0-100) that calibration
	// metrics and CLI simulation evaluate against — keep these in sync so
	// /v1/metrics/calibration and trustgraph-simulate report the same cutoff.
	EnforcementThreshold int `envconfig:"ENFORCEMENT_THRESHOLD" default:"80"`

	// Plane B: LinkedIn OAuth. Credentials come from the environment only;
	// when ClientID/Secret are empty the OAuth endpoints return 503 rather
	// than accepting unauthenticated verification.
	LinkedInClientID     string `envconfig:"LINKEDIN_CLIENT_ID" default:""`
	LinkedInClientSecret string `envconfig:"LINKEDIN_CLIENT_SECRET" default:""`
	LinkedInRedirectURI  string `envconfig:"LINKEDIN_REDIRECT_URI" default:""`

	// Plane B: identity verification vendor (Persona/Onfido-compatible REST).
	// When APIKey is empty the ID endpoints return 503.
	IDVendorAPIKey  string `envconfig:"ID_VENDOR_API_KEY" default:""`
	IDVendorBaseURL string `envconfig:"ID_VENDOR_BASE_URL" default:"https://withpersona.com/api/v1"`
	IDVendorName    string `envconfig:"ID_VENDOR_NAME" default:"persona"`

	// Plane B: liveness vendor. Falls back to the ID vendor settings when empty.
	LivenessVendorAPIKey  string `envconfig:"LIVENESS_VENDOR_API_KEY" default:""`
	LivenessVendorBaseURL string `envconfig:"LIVENESS_VENDOR_BASE_URL" default:""`

	// Plane B: reverse image search + synthetic detection.
	ReverseImageAPIKey  string `envconfig:"REVERSE_IMAGE_API_KEY" default:""`
	ReverseImageBaseURL string `envconfig:"REVERSE_IMAGE_BASE_URL" default:"https://saucenao.com/search.php"`
	SyntheticAPIKey     string `envconfig:"SYNTHETIC_IMAGE_API_KEY" default:""`
	SyntheticBaseURL    string `envconfig:"SYNTHETIC_IMAGE_BASE_URL" default:""`

	// Plane C: investigator authentication. When empty every investigator
	// endpoint returns 503 so the tools cannot be reached without an explicit
	// configuration. (An ADMIN_TOKEN is also accepted, at supervisor level.)
	InvestigatorToken string `envconfig:"INVESTIGATOR_TOKEN" default:""`

	// Plane C: break-glass grants last this long before expiring. Kept short by
	// default so emergency elevation cannot become a standing backdoor.
	BreakGlassDurationMinutes int `envconfig:"BREAK_GLASS_DURATION_MINUTES" default:"60"`

	// Plane C: OSINT tool configuration. TheHarvester/SpiderFoot are external
	// binaries unless a SpiderFoot server URL is given.
	HarvesterBinary   string `envconfig:"HARVESTER_BINARY" default:"theHarvester"`
	SpiderFootBaseURL string `envconfig:"SPIDERFOOT_BASE_URL" default:""`
	SpiderFootAPIKey  string `envconfig:"SPIDERFOOT_API_KEY" default:""`
	SpiderFootBinary  string `envconfig:"SPIDERFOOT_BINARY" default:"spiderfoot"`
}

// LinkedInConfigured reports whether the OAuth flow has credentials.
func (c *Config) LinkedInConfigured() bool {
	return c.LinkedInClientID != "" && c.LinkedInClientSecret != "" && c.LinkedInRedirectURI != ""
}

// IDVendorConfigured reports whether government-ID vendor calls are possible.
func (c *Config) IDVendorConfigured() bool { return c.IDVendorAPIKey != "" }

// LivenessVendorConfigured reports whether liveness calls are possible.
func (c *Config) LivenessVendorConfigured() bool {
	return c.LivenessVendorAPIKey != "" || c.IDVendorAPIKey != ""
}

// ReverseImageConfigured reports whether reverse image search is possible.
func (c *Config) ReverseImageConfigured() bool { return c.ReverseImageAPIKey != "" }

// SyntheticConfigured reports whether synthetic-image detection is possible.
func (c *Config) SyntheticConfigured() bool { return c.SyntheticAPIKey != "" }

// InvestigatorsConfigured reports whether investigator access is provisioned.
func (c *Config) InvestigatorsConfigured() bool { return c.InvestigatorToken != "" }
