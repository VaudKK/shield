// Package config loads Shield's runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env         string
	Port        string
	DatabaseURL string

	S3Endpoint  string
	S3Region    string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string

	OpenAIAPIKey string
	// OpenAIModel overrides the default model used for evidence analysis.
	OpenAIModel string

	// NudeNetServiceURL points at the self-hosted content-safety service.
	// Empty disables content-safety classification: uploads stay
	// "quarantined" rather than being claimed safe with nothing checked.
	NudeNetServiceURL string

	CORSAllowedOrigins []string

	SessionSecret string
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:                getEnv("APP_ENV", "development"),
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		S3Endpoint:         os.Getenv("S3_ENDPOINT"),
		S3Region:           getEnv("S3_REGION", "auto"),
		S3Bucket:           os.Getenv("S3_BUCKET"),
		S3AccessKey:        os.Getenv("S3_ACCESS_KEY_ID"),
		S3SecretKey:        os.Getenv("S3_SECRET_ACCESS_KEY"),
		OpenAIAPIKey:       os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:        os.Getenv("OPENAI_MODEL"),
		NudeNetServiceURL:  os.Getenv("NUDENET_SERVICE_URL"),
		SessionSecret:      os.Getenv("SESSION_SECRET"),
		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
