package env
import "github.com/caarlos0/env/v11"
type osEnvConfig struct {
	URL string `env:"OPENSEARCH_URL,required"`
	MessagesIndex string `env:"OPENSEARCH_MESSAGES_INDEX" envDefault:"messages"`
}
type osConfig struct{ raw osEnvConfig }
func NewOpenSearchConfig() (*osConfig, error) { var raw osEnvConfig; if err := env.Parse(&raw); err != nil { return nil, err }; return &osConfig{raw: raw}, nil }
func (c *osConfig) URL() string { return c.raw.URL }
func (c *osConfig) MessagesIndex() string { return c.raw.MessagesIndex }
