package mwkafka

import (
	"context"

	"go.uber.org/zap"

	kafkapkg "github.com/CARAI-REA/messanger/platform/pkg/kafka"
	"github.com/CARAI-REA/messanger/platform/pkg/kafka/consumer"
)

type Logger interface {
	Info(ctx context.Context, msg string, fields ...zap.Field)
}

// Logging — middleware логирования входящих сообщений (как в clean_arch).
func Logging(logger Logger) consumer.Middleware {
	return func(next kafkapkg.MessageHandler) kafkapkg.MessageHandler {
		return func(ctx context.Context, msg kafkapkg.Message) error {
			logger.Info(ctx, "Kafka msg received", zap.String("topic", msg.Topic))
			return next(ctx, msg)
		}
	}
}
