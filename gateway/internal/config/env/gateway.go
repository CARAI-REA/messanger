package env

import (
	"github.com/caarlos0/env/v11"
	"strings"
	"time"
)

type gatewayEnvConfig struct {
	RealtimeChannel  string        `env:"REALTIME_CHANNEL" envDefault:"chat:realtime"`
	AllowedOrigins   string        `env:"WS_ALLOWED_ORIGINS" envDefault:"*"`
	PresenceTTL      time.Duration `env:"PRESENCE_TTL" envDefault:"45s"`
	TypingTTL        time.Duration `env:"TYPING_TTL" envDefault:"5s"`
	ChatGRPCAddress  string        `env:"CHAT_GRPC_ADDRESS" envDefault:"localhost:50052"`
	ChatGRPCTLS      bool          `env:"CHAT_GRPC_TLS" envDefault:"false"`
	ChatGRPCCAFile   string        `env:"CHAT_GRPC_CA_FILE"`
	ChatGRPCServerName string      `env:"CHAT_GRPC_SERVER_NAME" envDefault:"chat"`
	AllowQueryToken  bool          `env:"ALLOW_QUERY_TOKEN" envDefault:"true"`
	MaxWSConnections int           `env:"MAX_WS_CONNECTIONS" envDefault:"50000"`
	MaxMsgPerSec     int           `env:"MAX_MSG_PER_SEC" envDefault:"20"`
	DrainSeconds     int           `env:"DRAIN_SECONDS" envDefault:"20"`
}
type gatewayConfig struct{ raw gatewayEnvConfig }

func NewGatewayConfig() (*gatewayConfig, error) {
	var raw gatewayEnvConfig
	if err := env.Parse(&raw); err != nil {
		return nil, err
	}
	return &gatewayConfig{raw: raw}, nil
}
func (c *gatewayConfig) RealtimeChannel() string    { return c.raw.RealtimeChannel }
func (c *gatewayConfig) PresenceTTL() time.Duration { return c.raw.PresenceTTL }
func (c *gatewayConfig) TypingTTL() time.Duration   { return c.raw.TypingTTL }
func (c *gatewayConfig) ChatGRPCAddress() string    { return c.raw.ChatGRPCAddress }
func (c *gatewayConfig) ChatGRPCTLS() bool          { return c.raw.ChatGRPCTLS }
func (c *gatewayConfig) ChatGRPCCAFile() string     { return c.raw.ChatGRPCCAFile }
func (c *gatewayConfig) ChatGRPCServerName() string { return c.raw.ChatGRPCServerName }
func (c *gatewayConfig) AllowQueryToken() bool      { return c.raw.AllowQueryToken }
func (c *gatewayConfig) MaxWSConnections() int      { return c.raw.MaxWSConnections }
func (c *gatewayConfig) MaxMsgPerSec() int          { return c.raw.MaxMsgPerSec }
func (c *gatewayConfig) DrainDuration() time.Duration {
	if c.raw.DrainSeconds <= 0 {
		return 0
	}
	return time.Duration(c.raw.DrainSeconds) * time.Second
}
func (c *gatewayConfig) AllowedOrigins() []string {
	parts := strings.Split(c.raw.AllowedOrigins, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
