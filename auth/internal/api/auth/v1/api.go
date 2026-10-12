package authv1api

import (
	"context"
	"strings"

	authv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/auth/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	authsvc "auth/internal/service/auth"
)

type Implementation struct {
	authv1.UnimplementedAuthServiceServer
	svc *authsvc.Service
}

func New(svc *authsvc.Service) *Implementation { return &Implementation{svc: svc} }

func (i *Implementation) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	refresh, access, err := i.svc.Login(ctx, req.Email, req.Password)
	if err != nil { return nil, status.Errorf(codes.Unauthenticated, "%v", err) }
	return &authv1.LoginResponse{RefreshToken: refresh, AccessToken: access}, nil
}

func (i *Implementation) GetRefreshToken(ctx context.Context, req *authv1.GetRefreshTokenRequest) (*authv1.GetRefreshTokenResponse, error) {
	tok, err := i.svc.Refresh(ctx, req.RefreshToken)
	if err != nil { return nil, status.Errorf(codes.Unauthenticated, "%v", err) }
	return &authv1.GetRefreshTokenResponse{RefreshToken: tok}, nil
}

func (i *Implementation) GetAccessToken(ctx context.Context, req *authv1.GetAccessTokenRequest) (*authv1.GetAccessTokenResponse, error) {
	tok, err := i.svc.Access(ctx, req.RefreshToken)
	if err != nil { return nil, status.Errorf(codes.Unauthenticated, "%v", err) }
	return &authv1.GetAccessTokenResponse{AccessToken: tok}, nil
}

func (i *Implementation) ValidateToken(ctx context.Context, _ *authv1.ValidateTokenRequest) (*emptypb.Empty, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	vals := md.Get("authorization")
	if len(vals) == 0 { return nil, status.Error(codes.Unauthenticated, "missing token") }
	tok := strings.TrimPrefix(vals[0], "Bearer ")
	if err := i.svc.ValidateAccess(ctx, tok); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	return &emptypb.Empty{}, nil
}

func (i *Implementation) Logout(ctx context.Context, req *authv1.LogoutRequest) (*emptypb.Empty, error) {
	if err := i.svc.Logout(ctx, req.RefreshToken); err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}
	return &emptypb.Empty{}, nil
}
