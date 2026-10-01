package env
import ("fmt"; "time"; "github.com/caarlos0/env/v11")
type RedisConfig struct {
	Host string `env:"REDIS_HOST,required"`
	Port string `env:"REDIS_PORT" envDefault:"6379"`
	Password string `env:"REDIS_PASSWORD"`
	ConnTimeout time.Duration `env:"REDIS_CONNECTION_TIMEOUT" envDefault:"3s"`
	MaxIdle int `env:"REDIS_MAX_IDLE" envDefault:"10"`
	IdleTimeout time.Duration `env:"REDIS_IDLE_TIMEOUT" envDefault:"2m"`
	LoginMaxAttempts int `env:"LOGIN_MAX_ATTEMPTS" envDefault:"5"`
	LoginWindow time.Duration `env:"LOGIN_ATTEMPTS_WINDOW" envDefault:"15m"`
}
func NewRedisConfig() (*RedisConfig, error) { var c RedisConfig; return &c, env.Parse(&c) }
func (c *RedisConfig) Address() string { return fmt.Sprintf("%s:%s", c.Host, c.Port) }
