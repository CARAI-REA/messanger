package env

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type grpcEnvConfig struct {
	Host         string `env:"GRPC_HOST" envDefault:"0.0.0.0"`
	Port         string `env:"GRPC_PORT" envDefault:"50055"`
	TLSCertFile  string `env:"GRPC_TLS_CERT_FILE"`
	TLSKeyFile   string `env:"GRPC_TLS_KEY_FILE"`
	TLSClientCA  string `env:"GRPC_TLS_CLIENT_CA"`
}
type grpcConfig struct{ raw grpcEnvConfig }

func NewGRPCConfig() (*grpcConfig, error) {
	var raw grpcEnvConfig
	if err := env.Parse(&raw); err != nil {
		return nil, err
	}
	return &grpcConfig{raw: raw}, nil
}
func (c *grpcConfig) Host() string         { return c.raw.Host }
func (c *grpcConfig) Port() string         { return c.raw.Port }
func (c *grpcConfig) Address() string      { return fmt.Sprintf("%s:%s", c.raw.Host, c.raw.Port) }
func (c *grpcConfig) TLSCertFile() string  { return c.raw.TLSCertFile }
func (c *grpcConfig) TLSKeyFile() string   { return c.raw.TLSKeyFile }
func (c *grpcConfig) TLSClientCA() string  { return c.raw.TLSClientCA }
