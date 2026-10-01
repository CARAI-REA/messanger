package logger

import "context"

// ContextWithUserID кладёт user_id в контекст для structured logging.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// ContextWithTraceID кладёт trace_id / request_id в контекст для structured logging.
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}
