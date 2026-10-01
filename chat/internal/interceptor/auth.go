package interceptor

import (
	"context"
	"strconv"
	"strings"

	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	chatv1api "chat/internal/api/chat/v1"
)

var serviceProxyMethods = map[string]bool{
	"/chat.v1.ChatService/ListChats":   true,
	"/chat.v1.ChatService/ListChatIDs": true,
}

func AuthInterceptor(access tokens.AccessTokenVerifier, service tokens.ServiceTokenVerifier) grpc.UnaryServerInterceptor {
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

		if serviceProxyMethods[info.FullMethod] {
			claims, err := service.VerifyServiceToken(ctx, token)
			if err == nil && claims.Service == "gateway" {
				userVals := md.Get("x-user-id")
				if len(userVals) == 0 {
					return nil, status.Error(codes.Unauthenticated, "missing x-user-id")
				}
				uid, err := strconv.ParseInt(userVals[0], 10, 64)
				if err != nil || uid == 0 {
					return nil, status.Error(codes.Unauthenticated, "invalid x-user-id")
				}
				ctx = chatv1api.WithUserID(ctx, uid)
				return handler(ctx, req)
			}
		}

		claims, err := access.VerifyAccessToken(ctx, token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid access token")
		}
		uid, err := chatv1api.ParseUserIDString(claims.UserUUID)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid user_id claim")
		}
		ctx = chatv1api.WithUserID(ctx, uid)
		return handler(ctx, req)
	}
}
