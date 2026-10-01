package env

import "github.com/caarlos0/env/v11"

type pushEnvConfig struct {
	Provider        string `env:"PUSH_PROVIDER" envDefault:"mock"`
	CredentialsFile string `env:"FCM_CREDENTIALS_FILE"`
}
type pushConfig struct{ raw pushEnvConfig }

func NewPushConfig() (*pushConfig, error) {
	var raw pushEnvConfig
	if err := env.Parse(&raw); err != nil {
		return nil, err
	}
	return &pushConfig{raw: raw}, nil
}
func (c *pushConfig) Provider() string        { return c.raw.Provider }
func (c *pushConfig) CredentialsFile() string { return c.raw.CredentialsFile }
