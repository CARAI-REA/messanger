package interceptor

import (
	"context"
	"strings"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/logger"
	"github.com/google/uuid"
	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	notifyv1api "notify/internal/api/notify/v1"
)

func AuthInterceptor(access tokens.AccessTokenVerifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}
		vals := md.Get("authorization")
		if len(vals) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization")
		}
		token := strings.TrimPrefix(vals[0], "Bearer ")
		token = strings.TrimPrefix(token, "bearer ")
		claims, err := access.VerifyAccessToken(ctx, token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid access token")
		}
		uid, err := notifyv1api.ParseUserIDString(claims.UserUUID)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid user_id claim")
		}
		return handler(notifyv1api.WithUserID(ctx, uid), req)
	}
}

func LoggerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		requestID := uuid.NewString()
		ctx = logger.ContextWithTraceID(ctx, requestID)
		start := time.Now()
		resp, err := handler(ctx, req)
		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.String("request_id", requestID),
			zap.Duration("duration", time.Since(start)),
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
