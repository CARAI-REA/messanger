package env
import "github.com/caarlos0/env/v11"
type LoggerConfig struct {
	Lvl string `env:"LOGGER_LEVEL" envDefault:"info"`
	JSON bool `env:"LOGGER_AS_JSON" envDefault:"true"`
}
func NewLoggerConfig() (*LoggerConfig, error) { var c LoggerConfig; return &c, env.Parse(&c) }
func (c *LoggerConfig) Level() string { return c.Lvl }
func (c *LoggerConfig) AsJson() bool { return c.JSON }
