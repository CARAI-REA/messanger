package env

import "github.com/caarlos0/env/v11"

type UserClientConfig struct {
	Address string `env:"USER_GRPC_ADDRESS,required"`
	TLS     bool   `env:"USER_GRPC_TLS" envDefault:"false"`
	CAFile  string `env:"USER_GRPC_CA_FILE"`
	ServerName string `env:"USER_GRPC_SERVER_NAME" envDefault:"user"`
}

func NewUserClientConfig() (*UserClientConfig, error) {
	var c UserClientConfig
	return &c, env.Parse(&c)
}
