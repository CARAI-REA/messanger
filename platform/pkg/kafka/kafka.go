package kafka

import (
	"context"
)

// MessageHandler — обработчик сообщений.
type MessageHandler func(ctx context.Context, msg Message) error

// Consumer читает сообщения из Kafka (consumer group).
type Consumer interface {
	Consume(ctx context.Context, handler MessageHandler) error
}

// Producer публикует сообщения в один топик.
type Producer interface {
	Send(ctx context.Context, key, value []byte) error
}
