package interceptor

import (
	"context"
	"path"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func LoggerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		requestID := uuid.NewString()
		ctx = logger.ContextWithTraceID(ctx, requestID)
		start := time.Now()
		resp, err := handler(ctx, req)
		fields := []zap.Field{
			zap.String("method", path.Base(info.FullMethod)),
			zap.String("request_id", requestID),
			zap.Duration("duration", time.Since(start)),
			zap.String("code", status.Code(err).String()),
		}
		if err != nil {
			fields = append(fields, zap.Error(err))
			logger.Error(ctx, "grpc finished", fields...)
		} else {
			logger.Info(ctx, "grpc finished", fields...)
		}
		return resp, err
	}
}
