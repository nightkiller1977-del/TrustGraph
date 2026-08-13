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
}
