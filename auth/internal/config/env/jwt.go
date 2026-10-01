package env

import (
	"time"

	"github.com/caarlos0/env/v11"
)

type JWTConfig struct {
	Secret           string        `env:"JWT_SECRET,required"`
	RefreshSecret    string        `env:"REFRESH_TOKEN_SECRET,required"`
	ServiceSecret    string        `env:"SERVICE_JWT_SECRET,required"`
	AccessTokenTTL   time.Duration `env:"ACCESS_TOKEN_TTL" envDefault:"15m"`
	RefreshTokenTTL  time.Duration `env:"REFRESH_TOKEN_TTL" envDefault:"168h"`
}

func NewJWTConfig() (*JWTConfig, error) {
	var c JWTConfig
	return &c, env.Parse(&c)
}

func (c *JWTConfig) AuthTokenSecretKey() string    { return c.Secret }
func (c *JWTConfig) RefreshTokenSecretKey() string { return c.RefreshSecret }
func (c *JWTConfig) ServiceTokenSecretKey() string { return c.ServiceSecret }
func (c *JWTConfig) AccessTTL() time.Duration      { return c.AccessTokenTTL }
func (c *JWTConfig) RefreshTTL() time.Duration     { return c.RefreshTokenTTL }
