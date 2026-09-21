package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port            string
	DatabaseURL     string
	APIKey          string
	CouponIndexPath string
	LogLevel        string
	CORSOrigin      string
}

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
