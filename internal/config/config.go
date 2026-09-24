// Package config reads server settings from environment variables, with
// defaults for local development.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds the server settings; see Load for the variables and defaults.
type Config struct {
	Port            string
	DatabaseURL     string
	APIKey          string
	CouponIndexPath string
	LogLevel        string
	CORSOrigin      string

	// JWTSecret signs access tokens. Empty means the server generates a
	// random one at startup.
	JWTSecret string
	// JWTTTL is how long an access token stays valid.
	JWTTTL time.Duration
	// AuthUsername and AuthPassword are the demo user's credentials for
	// POST /auth/token.
	AuthUsername string
	AuthPassword string
}

// Load reads Config from the environment. An unset or empty variable falls
// back to its default; a JWT_TTL that is not a positive Go duration is an
// error.
func Load() (Config, error) {
	ttl, err := time.ParseDuration(getEnv("JWT_TTL", "15m"))
	if err != nil || ttl <= 0 {
		return Config{}, fmt.Errorf("config: JWT_TTL must be a positive duration such as 15m, got %q", os.Getenv("JWT_TTL"))
	}
	return Config{
		Port:            getEnv("PORT", "8080"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://oolio:oolio@localhost:5432/oolio?sslmode=disable"),
		APIKey:          getEnv("API_KEY", "apitest"),
		CouponIndexPath: getEnv("COUPON_INDEX_PATH", "coupons/coupons.idx"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		CORSOrigin:      getEnv("CORS_ALLOWED_ORIGIN", "*"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		JWTTTL:          ttl,
		AuthUsername:    getEnv("AUTH_USERNAME", "demo"),
		AuthPassword:    getEnv("AUTH_PASSWORD", "demo1234"),
	}, nil
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
