package env

import (
	"fmt"
	"github.com/caarlos0/env/v11"
)

type postgresEnvConfig struct {
	Host         string `env:"POSTGRES_HOST,required"`
	Port         string `env:"POSTGRES_PORT,required"`
	Database     string `env:"POSTGRES_DB,required"`
	User         string `env:"POSTGRES_USER,required"`
	Password     string `env:"POSTGRES_PASSWORD,required"`
	SSLMode      string `env:"POSTGRES_SSL_MODE" envDefault:"disable"`
	MigrationDir string `env:"MIGRATION_DIRECTORY"`
}
type postgresConfig struct{ raw postgresEnvConfig }

func NewPostgresConfig() (*postgresConfig, error) {
	var raw postgresEnvConfig
	if err := env.Parse(&raw); err != nil {
		return nil, err
	}
	return &postgresConfig{raw: raw}, nil
}
func (c *postgresConfig) URI() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.raw.User, c.raw.Password, c.raw.Host, c.raw.Port, c.raw.Database, c.raw.SSLMode)
}
func (c *postgresConfig) MigrationDir() string { return c.raw.MigrationDir }
func (c *postgresConfig) SSLMode() string      { return c.raw.SSLMode }
