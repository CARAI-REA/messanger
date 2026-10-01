package env

import (
	"fmt"
	"github.com/caarlos0/env/v11"
)

type httpEnvConfig struct {
	Host string `env:"HTTP_HOST" envDefault:"0.0.0.0"`
	Port string `env:"HTTP_PORT" envDefault:"8082"`
}
type httpConfig struct{ raw httpEnvConfig }

func NewHTTPConfig() (*httpConfig, error) {
	var raw httpEnvConfig
	if err := env.Parse(&raw); err != nil {
		return nil, err
	}
	return &httpConfig{raw: raw}, nil
}
func (c *httpConfig) Host() string    { return c.raw.Host }
func (c *httpConfig) Port() string    { return c.raw.Port }
func (c *httpConfig) Address() string { return fmt.Sprintf("%s:%s", c.raw.Host, c.raw.Port) }
