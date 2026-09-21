// Package config loads process configuration from environment variables,
// with sane defaults so the service runs locally with zero setup.
package config

import (
	"fmt"
	"os"
)

// Config holds all environment-derived settings for cmd/server.
type Config struct {
	Port            string
	DatabaseURL     string
	APIKey          string
	CouponIndexPath string
	LogLevel        string
	CORSOrigin      string
}

// Load reads Config from the environment, applying defaults for anything
// unset. It never fails: a missing DatabaseURL simply means the caller
// will get a clear connection error at startup instead of here.
func Load() Config {
	return Config{
		Port:            getEnv("PORT", "8080"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://oolio:oolio@localhost:5432/oolio?sslmode=disable"),
		APIKey:          getEnv("API_KEY", "apitest"),
		CouponIndexPath: getEnv("COUPON_INDEX_PATH", "coupons/coupons.idx"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		CORSOrigin:      getEnv("CORS_ALLOWED_ORIGIN", "*"),
	}
}

func (c Config) Addr() string {
	return fmt.Sprintf(":%s", c.Port)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
