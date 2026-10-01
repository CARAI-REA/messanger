package config

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
type OpenSearchConfig interface {
	URL() string
	MessagesIndex() string
}
