package config

import "time"

type AppEnvConfig interface {
	Env() string
}

type LoggerConfig interface {
	Level() string
	AsJson() bool
}

type HTTPConfig interface {
	Address() string
	Host() string
	Port() string
}

type JWTConfig interface {
	AuthTokenSecretKey() string
	ServiceTokenSecretKey() string
}

type MetricsConfig interface {
	Address() string
}

type RedisConfig interface {
	Address() string
	Password() string
	ConnTimeout() time.Duration
	MaxIdle() int
	IdleTimeout() time.Duration
}

type GatewayConfig interface {
	RealtimeChannel() string
	AllowedOrigins() []string
	PresenceTTL() time.Duration
	TypingTTL() time.Duration
	ChatGRPCAddress() string
	ChatGRPCTLS() bool
	ChatGRPCCAFile() string
	ChatGRPCServerName() string
	AllowQueryToken() bool
	MaxWSConnections() int
	MaxMsgPerSec() int
	DrainDuration() time.Duration
}
