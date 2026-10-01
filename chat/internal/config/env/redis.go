package env

import (
	"fmt"
	"time"
	"github.com/caarlos0/env/v11"
)

type redisEnvConfig struct {
	Host        string        `env:"REDIS_HOST,required"`
	Port        string        `env:"REDIS_PORT" envDefault:"6379"`
	Password    string        `env:"REDIS_PASSWORD"`
	ConnTimeout time.Duration `env:"REDIS_CONNECTION_TIMEOUT" envDefault:"3s"`
	MaxIdle     int           `env:"REDIS_MAX_IDLE" envDefault:"20"`
	IdleTimeout time.Duration `env:"REDIS_IDLE_TIMEOUT" envDefault:"2m"`
}
type redisConfig struct{ raw redisEnvConfig }

func NewRedisConfig() (*redisConfig, error) {
	var raw redisEnvConfig
	if err := env.Parse(&raw); err != nil {
		return nil, err
	}
	return &redisConfig{raw: raw}, nil
}
func (c *redisConfig) Address() string            { return fmt.Sprintf("%s:%s", c.raw.Host, c.raw.Port) }
func (c *redisConfig) Password() string           { return c.raw.Password }
func (c *redisConfig) ConnTimeout() time.Duration { return c.raw.ConnTimeout }
func (c *redisConfig) MaxIdle() int               { return c.raw.MaxIdle }
func (c *redisConfig) IdleTimeout() time.Duration { return c.raw.IdleTimeout }
