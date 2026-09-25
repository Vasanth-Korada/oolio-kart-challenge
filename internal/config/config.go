// Package config loads server settings from config/<APP_ENV>.json with viper,
// then applies environment variable overrides. Secrets are never kept in the
// stage and prod files: they must come from the environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Environments with a config file in the config directory.
const (
	EnvDev   = "dev"
	EnvStage = "stage"
	EnvProd  = "prod"
)

// Environments lists every supported APP_ENV value.
var Environments = []string{EnvDev, EnvStage, EnvProd}

// Defaults for the variables that choose which file to load.
const (
	defaultEnv = EnvDev
	defaultDir = "config"
)

// envOverrides maps each config key to the environment variable that
// overrides it. The names predate the config files, so existing deployments
// keep working. An empty variable counts as unset.
var envOverrides = map[string]string{
	"server.port":        "PORT",
	"log.level":          "LOG_LEVEL",
	"cors.allowedOrigin": "CORS_ALLOWED_ORIGIN",
	"database.url":       "DATABASE_URL",
	"coupon.indexPath":   "COUPON_INDEX_PATH",
	"auth.apiKey":        "API_KEY",
	"auth.jwtSecret":     "JWT_SECRET",
	"auth.jwtTTL":        "JWT_TTL",
	"auth.username":      "AUTH_USERNAME",
	"auth.password":      "AUTH_PASSWORD",
}

// Config holds the server settings. Field names match the JSON keys.
type Config struct {
	// Env is the APP_ENV this config was loaded for; File is the file read.
	Env  string `mapstructure:"-"`
	File string `mapstructure:"-"`

	Server   ServerConfig   `mapstructure:"server"`
	Log      LogConfig      `mapstructure:"log"`
	CORS     CORSConfig     `mapstructure:"cors"`
	Database DatabaseConfig `mapstructure:"database"`
	Coupon   CouponConfig   `mapstructure:"coupon"`
	Auth     AuthConfig     `mapstructure:"auth"`
}

// ServerConfig holds the listen port and the http.Server timeouts.
type ServerConfig struct {
	Port              string        `mapstructure:"port"`
	ReadHeaderTimeout time.Duration `mapstructure:"readHeaderTimeout"`
	ReadTimeout       time.Duration `mapstructure:"readTimeout"`
	WriteTimeout      time.Duration `mapstructure:"writeTimeout"`
	IdleTimeout       time.Duration `mapstructure:"idleTimeout"`
	// ShutdownTimeout bounds graceful shutdown after SIGINT or SIGTERM.
	ShutdownTimeout time.Duration `mapstructure:"shutdownTimeout"`
}

// LogConfig sets the minimum log level: debug, info, warn or error.
type LogConfig struct {
	Level string `mapstructure:"level"`
}

// CORSConfig sets the allowed browser origin; "" disables CORS headers.
type CORSConfig struct {
	AllowedOrigin string `mapstructure:"allowedOrigin"`
}

// DatabaseConfig holds the Postgres DSN and whether an unreachable database
// may fall back to in-memory storage (dev only).
type DatabaseConfig struct {
	URL              string `mapstructure:"url"`
	FallbackToMemory bool   `mapstructure:"fallbackToMemory"`
}

// CouponConfig holds the path of the coupon index file.
type CouponConfig struct {
	IndexPath string `mapstructure:"indexPath"`
}

// AuthConfig holds the API key, JWT settings and the demo user. An empty
// JWTSecret in dev means the server generates a random one.
type AuthConfig struct {
	APIKey    string        `mapstructure:"apiKey"`
	JWTSecret string        `mapstructure:"jwtSecret"`
	JWTTTL    time.Duration `mapstructure:"jwtTTL"`
	Username  string        `mapstructure:"username"`
	Password  string        `mapstructure:"password"`
}

// Load reads <CONFIG_DIR>/<APP_ENV>.json (defaults: config, dev), applies
// the environment overrides and validates the result. Unknown JSON keys are
// an error, so a typo can't silently fall back to a zero value.
func Load() (Config, error) {
	env := getEnv("APP_ENV", defaultEnv)
	if !slices.Contains(Environments, env) {
		return Config{}, fmt.Errorf("config: APP_ENV must be one of %s, got %q", strings.Join(Environments, ", "), env)
	}
	file := filepath.Join(getEnv("CONFIG_DIR", defaultDir), env+".json")

	settings := viper.New()
	settings.SetConfigFile(file)
	if err := settings.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("config: read %s: %w", file, err)
	}
	for key, envVar := range envOverrides {
		if err := settings.BindEnv(key, envVar); err != nil {
			return Config{}, fmt.Errorf("config: bind %s: %w", envVar, err)
		}
	}

	var cfg Config
	if err := settings.UnmarshalExact(&cfg); err != nil {
		return Config{}, fmt.Errorf("config: parse %s: %w", file, err)
	}
	cfg.Env, cfg.File = env, file

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks that durations are positive and required values are set.
// Outside dev, the database URL and every credential must be set, which in
// practice means supplied through the environment.
func (c Config) Validate() error {
	var errs []error
	if c.Server.Port == "" {
		errs = append(errs, errors.New("server.port (PORT) is required"))
	}
	if c.Coupon.IndexPath == "" {
		errs = append(errs, errors.New("coupon.indexPath (COUPON_INDEX_PATH) is required"))
	}
	durations := []struct {
		name  string
		value time.Duration
	}{
		{"server.readHeaderTimeout", c.Server.ReadHeaderTimeout},
		{"server.readTimeout", c.Server.ReadTimeout},
		{"server.writeTimeout", c.Server.WriteTimeout},
		{"server.idleTimeout", c.Server.IdleTimeout},
		{"server.shutdownTimeout", c.Server.ShutdownTimeout},
		{"auth.jwtTTL (JWT_TTL)", c.Auth.JWTTTL},
	}
	for _, duration := range durations {
		if duration.value <= 0 {
			errs = append(errs, fmt.Errorf("%s must be a positive duration such as 15m", duration.name))
		}
	}

	if c.Env != EnvDev {
		required := []struct{ value, name string }{
			{c.Database.URL, "DATABASE_URL"},
			{c.Auth.APIKey, "API_KEY"},
			{c.Auth.JWTSecret, "JWT_SECRET"},
			{c.Auth.Username, "AUTH_USERNAME"},
			{c.Auth.Password, "AUTH_PASSWORD"},
		}
		for _, requirement := range required {
			if requirement.value == "" {
				errs = append(errs, fmt.Errorf("%s must be set in %s", requirement.name, c.Env))
			}
		}
		if c.Database.FallbackToMemory {
			errs = append(errs, fmt.Errorf("database.fallbackToMemory is only allowed in %s", EnvDev))
		}
	}

	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("config: invalid %s config:\n%w", c.Env, err)
	}
	return nil
}

// Addr returns the listen address, e.g. ":8080".
func (c Config) Addr() string {
	return ":" + c.Server.Port
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
