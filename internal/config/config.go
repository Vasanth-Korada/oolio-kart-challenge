// Package config reads server settings from environment variables, with
// defaults for local development.
package config

import (
	"fmt"
	"os"
)

// Config holds the server settings; see Load for the variables and defaults.
type Config struct {
	Port            string
	DatabaseURL     string
	APIKey          string
	CouponIndexPath string
	LogLevel        string
	CORSOrigin      string
}

// Load reads Config from the environment. An unset or empty variable falls
// back to its default.
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

// Addr returns the listen address, e.g. ":8080".
func (c Config) Addr() string {
	return fmt.Sprintf(":%s", c.Port)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
