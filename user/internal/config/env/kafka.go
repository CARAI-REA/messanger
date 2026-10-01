package env

import (
	"strings"
	"github.com/caarlos0/env/v11"
)

type kafkaEnvConfig struct {
	Brokers           string `env:"KAFKA_BROKERS,required"`
	UserCreatedTopic  string `env:"KAFKA_USER_CREATED_TOPIC" envDefault:"user.created"`
	UserDeletedTopic  string `env:"KAFKA_USER_DELETED_TOPIC" envDefault:"user.deleted"`
}
type kafkaConfig struct{ raw kafkaEnvConfig }

func NewKafkaConfig() (*kafkaConfig, error) {
	var raw kafkaEnvConfig
	if err := env.Parse(&raw); err != nil {
		return nil, err
	}
	return &kafkaConfig{raw: raw}, nil
}
func (c *kafkaConfig) Brokers() []string {
	parts := strings.Split(c.raw.Brokers, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
func (c *kafkaConfig) UserCreatedTopic() string { return c.raw.UserCreatedTopic }
func (c *kafkaConfig) UserDeletedTopic() string { return c.raw.UserDeletedTopic }
