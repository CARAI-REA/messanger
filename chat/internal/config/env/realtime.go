package env

import "github.com/caarlos0/env/v11"

type realtimeEnvConfig struct {
	Channel string `env:"REALTIME_CHANNEL" envDefault:"chat:realtime"`
}
type realtimeConfig struct{ raw realtimeEnvConfig }

func NewRealtimeConfig() (*realtimeConfig, error) {
	var raw realtimeEnvConfig
	if err := env.Parse(&raw); err != nil {
		return nil, err
	}
	return &realtimeConfig{raw: raw}, nil
}
func (c *realtimeConfig) Channel() string { return c.raw.Channel }
