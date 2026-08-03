package config

type Config struct {
	// Server
	Port        string `envconfig:"PORT" default:"8080"`
	Environment string `envconfig:"ENVIRONMENT" default:"development"`

	// Database
	DatabaseURL string `envconfig:"DATABASE_URL" default:"postgres://trustgraph:trustgraph_dev_password@localhost:5432/trustgraph"`

	// Assessment service
	AssessmentTimeoutMS int `envconfig:"ASSESSMENT_TIMEOUT_MS" default:"300"`

	// Circuit breaker
	CircuitBreakerMaxFailures int `envconfig:"CIRCUIT_BREAKER_MAX_FAILURES" default:"5"`
	CircuitBreakerWindowMins  int `envconfig:"CIRCUIT_BREAKER_WINDOW_MINS" default:"5"`

	// Logging
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
}
