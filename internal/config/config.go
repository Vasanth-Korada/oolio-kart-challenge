// Package config loads process configuration from environment variables.
// Every setting has a sane default so the service can run locally with
// zero configuration, per twelve-factor practice.
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
		// The React UI lives in a separate repo/origin (see README).
		// "*" is fine for this assignment's demo purposes; a real
		// deployment would pin this to the frontend's actual origin.
		CORSOrigin: getEnv("CORS_ALLOWED_ORIGIN", "*"),
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
