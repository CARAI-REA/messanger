package env

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type GRPCConfig struct {
	Host        string `env:"GRPC_HOST" envDefault:"0.0.0.0"`
	Port        string `env:"GRPC_PORT" envDefault:"50050"`
	TLSCertFile string `env:"GRPC_TLS_CERT_FILE"`
	TLSKeyFile  string `env:"GRPC_TLS_KEY_FILE"`
	TLSClientCA string `env:"GRPC_TLS_CLIENT_CA"`
}

func NewGRPCConfig() (*GRPCConfig, error) {
	var c GRPCConfig
	if err := env.Parse(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *GRPCConfig) Address() string { return fmt.Sprintf("%s:%s", c.Host, c.Port) }
