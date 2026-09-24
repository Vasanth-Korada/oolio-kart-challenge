package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/config"
)

// repoConfigDir holds the committed dev, stage and prod files.
const repoConfigDir = "../../config"

// allEnvVars are cleared before each test so the developer's shell can't
// leak into the results. Empty counts as unset.
var allEnvVars = []string{
	"APP_ENV", "CONFIG_DIR", "PORT", "LOG_LEVEL", "CORS_ALLOWED_ORIGIN", "DATABASE_URL",
	"COUPON_INDEX_PATH", "API_KEY", "JWT_SECRET", "JWT_TTL", "AUTH_USERNAME", "AUTH_PASSWORD",
}

// validJSON is a complete dev file for tests that use a temp directory.
const validJSON = `{
  "server": {"port": "8080", "readHeaderTimeout": "5s", "readTimeout": "10s",
             "writeTimeout": "10s", "idleTimeout": "60s", "shutdownTimeout": "10s"},
  "log": {"level": "info"},
  "cors": {"allowedOrigin": "*"},
  "database": {"url": "postgres://localhost/oolio", "fallbackToMemory": true},
  "coupon": {"indexPath": "coupons/coupons.idx"},
  "auth": {"apiKey": "k", "jwtSecret": "", "jwtTTL": "15m", "username": "demo", "password": "pw"}
}`

type ConfigSuite struct {
	suite.Suite
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}

func (s *ConfigSuite) SetupTest() {
	for _, k := range allEnvVars {
		s.T().Setenv(k, "")
	}
}

// writeConfig writes <dir>/<env>.json and points CONFIG_DIR at dir.
func (s *ConfigSuite) writeConfig(env, body string) {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, env+".json"), []byte(body), 0o600))
	s.T().Setenv("CONFIG_DIR", dir)
}

// setSecrets supplies what stage and prod require from the environment.
func (s *ConfigSuite) setSecrets() {
	s.T().Setenv("DATABASE_URL", "postgres://db.internal/oolio")
	s.T().Setenv("API_KEY", "stage-key")
	s.T().Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	s.T().Setenv("AUTH_USERNAME", "ops")
	s.T().Setenv("AUTH_PASSWORD", "s3cret")
}

func (s *ConfigSuite) TestDevIsTheDefault() {
	s.T().Setenv("CONFIG_DIR", repoConfigDir)

	cfg, err := config.Load()

	s.Require().NoError(err)
	s.Equal(config.EnvDev, cfg.Env)
	s.Equal(filepath.Join(repoConfigDir, "dev.json"), cfg.File)
	s.Equal(":8080", cfg.Addr())
	s.Equal("debug", cfg.Log.Level)
	s.True(cfg.Database.FallbackToMemory)
	s.Equal("apitest", cfg.Auth.APIKey)
	s.Empty(cfg.Auth.JWTSecret, "dev generates a random secret")
	s.Equal(15*time.Minute, cfg.Auth.JWTTTL)
	s.Equal(5*time.Second, cfg.Server.ReadHeaderTimeout)
	s.Equal(10*time.Second, cfg.Server.ShutdownTimeout)
}

func (s *ConfigSuite) TestRepoFilesForEveryEnvironment() {
	tests := []struct {
		env          string
		wantTTL      time.Duration
		wantShutdown time.Duration
		wantOrigin   string
	}{
		{env: config.EnvStage, wantTTL: 15 * time.Minute, wantShutdown: 15 * time.Second, wantOrigin: "https://stage.kart.example.com"},
		{env: config.EnvProd, wantTTL: 10 * time.Minute, wantShutdown: 30 * time.Second, wantOrigin: "https://kart.example.com"},
	}
	for _, tt := range tests {
		s.Run(tt.env, func() {
			s.T().Setenv("CONFIG_DIR", repoConfigDir)
			s.T().Setenv("APP_ENV", tt.env)
			s.setSecrets()

			cfg, err := config.Load()

			s.Require().NoError(err)
			s.Equal(tt.env, cfg.Env)
			s.Equal("info", cfg.Log.Level)
			s.False(cfg.Database.FallbackToMemory, "stage and prod fail fast without Postgres")
			s.Equal(tt.wantTTL, cfg.Auth.JWTTTL)
			s.Equal(tt.wantShutdown, cfg.Server.ShutdownTimeout)
			s.Equal(tt.wantOrigin, cfg.CORS.AllowedOrigin)
			s.Equal("postgres://db.internal/oolio", cfg.Database.URL, "secrets come from the environment")
		})
	}
}

func (s *ConfigSuite) TestStageAndProdRequireSecretsFromEnv() {
	for _, env := range []string{config.EnvStage, config.EnvProd} {
		s.Run(env, func() {
			s.T().Setenv("CONFIG_DIR", repoConfigDir)
			s.T().Setenv("APP_ENV", env)

			_, err := config.Load()

			s.Require().Error(err)
			for _, name := range []string{"DATABASE_URL", "API_KEY", "JWT_SECRET", "AUTH_USERNAME", "AUTH_PASSWORD"} {
				s.Contains(err.Error(), name, "every missing variable is reported at once")
			}
		})
	}
}

func (s *ConfigSuite) TestEnvOverridesFile() {
	s.writeConfig("dev", validJSON)
	s.T().Setenv("PORT", "9090")
	s.T().Setenv("LOG_LEVEL", "warn")
	s.T().Setenv("JWT_TTL", "1h")
	s.T().Setenv("DATABASE_URL", "postgres://override/oolio")

	cfg, err := config.Load()

	s.Require().NoError(err)
	s.Equal("9090", cfg.Server.Port)
	s.Equal("warn", cfg.Log.Level)
	s.Equal(time.Hour, cfg.Auth.JWTTTL)
	s.Equal("postgres://override/oolio", cfg.Database.URL)
	s.Equal("k", cfg.Auth.APIKey, "keys without an override keep the file value")
}

func (s *ConfigSuite) TestEmptyEnvVarKeepsFileValue() {
	s.writeConfig("dev", validJSON)
	s.T().Setenv("PORT", "")

	cfg, err := config.Load()

	s.Require().NoError(err)
	s.Equal("8080", cfg.Server.Port)
}

func (s *ConfigSuite) TestErrors() {
	tests := []struct {
		name    string
		env     string
		body    string // "" means don't write a file
		setenv  map[string]string
		wantErr string
	}{
		{name: "unknown APP_ENV", env: "qa", wantErr: "APP_ENV must be one of dev, stage, prod"},
		{name: "missing file", env: "dev", wantErr: "dev.json"},
		{name: "invalid JSON", env: "dev", body: `{"server": `, wantErr: "read"},
		{name: "unknown key (typo)", env: "dev", body: `{"server": {"prot": "8080"}}`, wantErr: "prot"},
		{name: "bad duration in file", env: "dev", body: replace(validJSON, `"jwtTTL": "15m"`, `"jwtTTL": "fifteen"`), wantErr: "parse"},
		{name: "bad duration in env", env: "dev", body: validJSON, setenv: map[string]string{"JWT_TTL": "15"}, wantErr: "parse"},
		{name: "zero duration", env: "dev", body: replace(validJSON, `"shutdownTimeout": "10s"`, `"shutdownTimeout": "0s"`), wantErr: "server.shutdownTimeout must be a positive duration"},
		{name: "negative JWT_TTL", env: "dev", body: validJSON, setenv: map[string]string{"JWT_TTL": "-5m"}, wantErr: "auth.jwtTTL (JWT_TTL) must be a positive duration"},
		{name: "memory fallback outside dev", env: "prod", body: validJSON, wantErr: "database.fallbackToMemory is only allowed in dev"},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()
			if tt.body != "" {
				s.writeConfig(tt.env, tt.body)
			} else {
				s.T().Setenv("CONFIG_DIR", s.T().TempDir())
			}
			s.T().Setenv("APP_ENV", tt.env)
			for k, v := range tt.setenv {
				s.T().Setenv(k, v)
			}

			_, err := config.Load()

			s.Require().Error(err)
			s.Contains(err.Error(), tt.wantErr)
		})
	}
}

func replace(s, old, new string) string {
	return strings.Replace(s, old, new, 1)
}
