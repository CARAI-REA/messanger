package grpcmw

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// RateLimitInterceptor limits unary RPCs per peer address.
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
