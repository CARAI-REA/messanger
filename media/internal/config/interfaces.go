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
type S3Config interface {
	Endpoint() string
	AccessKey() string
	SecretKey() string
	Bucket() string
	UseSSL() bool
	PublicEndpoint() string
	MaxUploadBytes() int64
	AllowedMimePrefixes() []string
}
