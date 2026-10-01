package config

import "time"

type AppEnvConfig interface {
	Env() string
	GRPCMaxRPS() float64
}

type LoggerConfig interface {
	Level() string
	AsJson() bool
}

type GRPCConfig interface {
	Address() string
	Host() string
	Port() string
	TLSCertFile() string
	TLSKeyFile() string
	TLSClientCA() string
}

type PostgresConfig interface {
	URI() string
	MigrationDir() string
	SSLMode() string
}

type KafkaConfig interface {
	Brokers() []string
	UserCreatedTopic() string
	UserDeletedTopic() string
}

type JWTConfig interface {
	AuthTokenSecretKey() string
	ServiceTokenSecretKey() string
}

type MetricsConfig interface {
	Address() string
}

// silence unused import in case
var _ = time.Second
