package interceptor

import (
	"context"
	"strings"

	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	userv1api "user/internal/api/user/v1"
)

var publicMethods = map[string]bool{
	"/user.v1.UserService/Create": true,
}

var serviceOnlyMethods = map[string]bool{
	"/user.v1.UserService/ValidateCredentials": true,
}

func AuthInterceptor(access tokens.AccessTokenVerifier, service tokens.ServiceTokenVerifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}
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

		if serviceOnlyMethods[info.FullMethod] {
			claims, err := service.VerifyServiceToken(ctx, token)
			if err != nil || claims.Service != "auth" {
				return nil, status.Error(codes.PermissionDenied, "service auth required")
			}
			return handler(ctx, req)
		}

		claims, err := access.VerifyAccessToken(ctx, token)
		if err != nil {
			// allow service tokens for internal admin ops if needed later
			return nil, status.Error(codes.Unauthenticated, "invalid access token")
		}
		uid, err := userv1api.ParseUserIDString(claims.UserUUID)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid user_id claim")
		}
		ctx = userv1api.WithUserID(ctx, uid)
		return handler(ctx, req)
	}
}
