package env

import "github.com/caarlos0/env/v11"

type appEnvConfig struct {
	Env string `env:"APP_ENV" envDefault:"development"`
}
type appConfig struct{ raw appEnvConfig }

func NewAppConfig() (*appConfig, error) {
	var raw appEnvConfig
	if err := env.Parse(&raw); err != nil {
		return nil, err
	}
	return &appConfig{raw: raw}, nil
}
func (c *appConfig) Env() string { return c.raw.Env }
