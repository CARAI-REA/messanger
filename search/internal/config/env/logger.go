package env
import "github.com/caarlos0/env/v11"
type loggerEnvConfig struct {
	Level string `env:"LOGGER_LEVEL" envDefault:"info"`
	AsJson bool `env:"LOGGER_AS_JSON" envDefault:"true"`
}
type loggerConfig struct{ raw loggerEnvConfig }
func NewLoggerConfig() (*loggerConfig, error) { var raw loggerEnvConfig; if err := env.Parse(&raw); err != nil { return nil, err }; return &loggerConfig{raw: raw}, nil }
func (c *loggerConfig) Level() string { return c.raw.Level }
func (c *loggerConfig) AsJson() bool { return c.raw.AsJson }
