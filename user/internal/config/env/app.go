package env

import "github.com/caarlos0/env/v11"

type appEnvConfig struct {
	Env        string  `env:"APP_ENV" envDefault:"development"`
	GRPCMaxRPS float64 `env:"GRPC_MAX_RPS" envDefault:"100"`
}

type appConfig struct{ raw appEnvConfig }

func NewAppConfig() (*appConfig, error) {
	var raw appEnvConfig
	if err := env.Parse(&raw); err != nil {
		return nil, err
	}
	return &appConfig{raw: raw}, nil
}
func (c *appConfig) Env() string         { return c.raw.Env }
func (c *appConfig) GRPCMaxRPS() float64 { return c.raw.GRPCMaxRPS }
