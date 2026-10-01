package env
import ("fmt"; "github.com/caarlos0/env/v11")
type MetricsConfig struct {
	Host string `env:"METRICS_HOST" envDefault:"0.0.0.0"`
	Port string `env:"METRICS_PORT" envDefault:"9102"`
}
func NewMetricsConfig() (*MetricsConfig, error) { var c MetricsConfig; return &c, env.Parse(&c) }
func (c *MetricsConfig) Address() string { return fmt.Sprintf("%s:%s", c.Host, c.Port) }
