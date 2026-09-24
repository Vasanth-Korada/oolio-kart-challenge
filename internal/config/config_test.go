package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/config"
)

type ConfigSuite struct {
	suite.Suite
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}

func (s *ConfigSuite) TestAuthDefaults() {
	for _, k := range []string{"JWT_SECRET", "JWT_TTL", "AUTH_USERNAME", "AUTH_PASSWORD"} {
		s.T().Setenv(k, "")
	}

	cfg, err := config.Load()

	s.Require().NoError(err)
	s.Empty(cfg.JWTSecret, "no default secret: the server generates one")
	s.Equal(15*time.Minute, cfg.JWTTTL)
	s.Equal("demo", cfg.AuthUsername)
	s.Equal("demo1234", cfg.AuthPassword)
}

func (s *ConfigSuite) TestAuthOverrides() {
	s.T().Setenv("JWT_SECRET", "s3cret")
	s.T().Setenv("JWT_TTL", "1h")
	s.T().Setenv("AUTH_USERNAME", "alice")
	s.T().Setenv("AUTH_PASSWORD", "pw")

	cfg, err := config.Load()

	s.Require().NoError(err)
	s.Equal("s3cret", cfg.JWTSecret)
	s.Equal(time.Hour, cfg.JWTTTL)
	s.Equal("alice", cfg.AuthUsername)
	s.Equal("pw", cfg.AuthPassword)
}

func (s *ConfigSuite) TestInvalidJWTTTL() {
	for _, v := range []string{"fifteen", "15", "-5m", "0s"} {
		s.Run(v, func() {
			s.T().Setenv("JWT_TTL", v)
			_, err := config.Load()
			s.Error(err)
		})
	}
}
