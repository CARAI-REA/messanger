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
	TLSCertFile() string
	TLSKeyFile() string
	TLSClientCA() string
}
type PostgresConfig interface {
	URI() string
	MigrationDir() string
	SSLMode() string
}
type JWTConfig interface {
	AuthTokenSecretKey() string
	ServiceTokenSecretKey() string
}
type MetricsConfig interface {
	Address() string
}
type KafkaConfig interface {
	Brokers() []string
	ChatEventsTopic() string
	GroupID() string
}
type PushConfig interface {
	Provider() string
	CredentialsFile() string
}
type RedisConfig interface {
	Enabled() bool
	Address() string
	Password() string
	ConnTimeout() time.Duration
	MaxIdle() int
	IdleTimeout() time.Duration
}
