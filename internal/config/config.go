package config

import (
	"os"
	"time"
)

// PortfolioAPI holds env for cmd/portfolio-api.
type PortfolioAPI struct {
	HTTPAddr       string
	DatabaseURL    string
	AuthUsername   string
	AuthPassword   string
	InternalAPIKey string
	SessionTTL     time.Duration
}

// LoadPortfolioAPI loads portfolio-api configuration from the environment.
func LoadPortfolioAPI() PortfolioAPI {
	return PortfolioAPI{
		HTTPAddr:       getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		AuthUsername:   getenv("AUTH_USERNAME", "admin"),
		AuthPassword:   getenv("AUTH_PASSWORD", "changeme"),
		InternalAPIKey: os.Getenv("INTERNAL_API_KEY"),
		SessionTTL:     getenvDuration("SESSION_TTL", 24*time.Hour),
	}
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func getenvDuration(k string, def time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
