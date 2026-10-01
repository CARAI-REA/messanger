package env
import "github.com/caarlos0/env/v11"
type jwtEnvConfig struct {
	AuthTokenSecretKey string `env:"JWT_SECRET,required"`
	ServiceTokenSecretKey string `env:"SERVICE_JWT_SECRET,required"`
}
type jwtConfig struct{ raw jwtEnvConfig }
func NewJWTConfig() (*jwtConfig, error) { var raw jwtEnvConfig; if err := env.Parse(&raw); err != nil { return nil, err }; return &jwtConfig{raw: raw}, nil }
func (c *jwtConfig) AuthTokenSecretKey() string { return c.raw.AuthTokenSecretKey }
func (c *jwtConfig) ServiceTokenSecretKey() string { return c.raw.ServiceTokenSecretKey }
