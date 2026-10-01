package env
import ("fmt"; "github.com/caarlos0/env/v11")
type metricsEnvConfig struct {
	Host string `env:"METRICS_HOST" envDefault:"0.0.0.0"`
	Port string `env:"METRICS_PORT" envDefault:"9105"`
}
type metricsConfig struct{ raw metricsEnvConfig }
func NewMetricsConfig() (*metricsConfig, error) { var raw metricsEnvConfig; if err := env.Parse(&raw); err != nil { return nil, err }; return &metricsConfig{raw: raw}, nil }
func (c *metricsConfig) Address() string { return fmt.Sprintf("%s:%s", c.raw.Host, c.raw.Port) }
