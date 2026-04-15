package config

import (
	"os"
	"strings"
	"time"
)

// PortfolioAPI holds env for cmd/portfolio-api.
type PortfolioAPI struct {
	HTTPAddr           string
	DatabaseURL        string
	AuthUsername       string
	AuthPassword       string
	InternalAPIKey     string
	SessionTTL         time.Duration
	CORSAllowedOrigins []string
}

// LoadPortfolioAPI loads portfolio-api configuration from the environment.
func LoadPortfolioAPI() PortfolioAPI {
	return PortfolioAPI{
		HTTPAddr:           getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		AuthUsername:       getenv("AUTH_USERNAME", "admin"),
		AuthPassword:       getenv("AUTH_PASSWORD", "changeme"),
		InternalAPIKey:     os.Getenv("INTERNAL_API_KEY"),
		SessionTTL:         getenvDuration("SESSION_TTL", 24*time.Hour),
		CORSAllowedOrigins: getenvCSVList("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000"),
	}
}

func getenvCSVList(k, def string) []string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		v = def
	}
	out := splitCSV(v)
	if len(out) == 0 {
		out = splitCSV(def)
	}
	return out
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
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
