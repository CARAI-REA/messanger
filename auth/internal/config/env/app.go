package env
import "github.com/caarlos0/env/v11"
type AppConfig struct {
	RawEnv string `env:"APP_ENV" envDefault:"development"`
	RPS float64 `env:"GRPC_MAX_RPS" envDefault:"50"`
}
func NewAppConfig() (*AppConfig, error) { var c AppConfig; return &c, env.Parse(&c) }
func (c *AppConfig) Env() string { return c.RawEnv }
func (c *AppConfig) GRPCMaxRPS() float64 { return c.RPS }
