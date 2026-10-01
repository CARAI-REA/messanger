package env
import ("strings"; "github.com/caarlos0/env/v11")
type kafkaEnvConfig struct {
	Brokers string `env:"KAFKA_BROKERS,required"`
	ChatEventsTopic string `env:"KAFKA_CHAT_EVENTS_TOPIC" envDefault:"chat.events"`
	GroupID string `env:"KAFKA_GROUP_ID" envDefault:"notify-chat-events"`
}
type kafkaConfig struct{ raw kafkaEnvConfig }
func NewKafkaConfig() (*kafkaConfig, error) { var raw kafkaEnvConfig; if err := env.Parse(&raw); err != nil { return nil, err }; return &kafkaConfig{raw: raw}, nil }
func (c *kafkaConfig) Brokers() []string {
	parts := strings.Split(c.raw.Brokers, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" { out = append(out, p) }
	}
	return out
}
func (c *kafkaConfig) ChatEventsTopic() string { return c.raw.ChatEventsTopic }
func (c *kafkaConfig) GroupID() string { return c.raw.GroupID }
