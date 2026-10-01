package interceptor

import (
	"context"
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/logger"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

var (
	requestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_grpc_requests_total",
		Help: "Auth gRPC requests",
	}, []string{"method", "code"})
	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "auth_grpc_request_duration_seconds",
		Help:    "Auth gRPC request duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"})
)

func init() {
	prometheus.MustRegister(requestsTotal, requestDuration)
}

func LoggerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		method := path.Base(info.FullMethod)
		requestID := uuid.NewString()
		ctx = logger.ContextWithTraceID(ctx, requestID)
		logger.Info(ctx, "Started gRPC method", zap.String("service", "auth"), zap.String("method", method), zap.String("request_id", requestID))
		start := time.Now()
		resp, err := handler(ctx, req)
		fields := []zap.Field{zap.String("method", method), zap.Duration("duration", time.Since(start))}
		if err != nil {
			st, _ := status.FromError(err)
			fields = append(fields, zap.String("code", st.Code().String()), zap.Error(err))
			logger.Error(ctx, fmt.Sprintf("Finished gRPC method %s", method), fields...)
		} else {
			fields = append(fields, zap.String("code", "OK"))
			logger.Info(ctx, fmt.Sprintf("Finished gRPC method %s", method), fields...)
		}
		return resp, err
	}
}

func MetricsInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		method := path.Base(info.FullMethod)
		start := time.Now()
		resp, err := handler(ctx, req)
		code := status.Code(err).String()
		requestsTotal.WithLabelValues(method, code).Inc()
		requestDuration.WithLabelValues(method).Observe(time.Since(start).Seconds())
		return resp, err
	}
}

func RateLimitInterceptor(maxRPS float64) grpc.UnaryServerInterceptor {
	if maxRPS <= 0 {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}
	burst := int(maxRPS)
	if burst < 1 {
		burst = 1
	}
	var mu sync.Mutex
	limiters := make(map[string]*rate.Limiter)
	get := func(key string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		if lim, ok := limiters[key]; ok {
			return lim
		}
		lim := rate.NewLimiter(rate.Limit(maxRPS), burst)
		limiters[key] = lim
		return lim
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		key := "peer:unknown"
		if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
			key = "peer:" + p.Addr.String()
		}
		if !get(key).Allow() {
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}
		return handler(ctx, req)
	}
}
