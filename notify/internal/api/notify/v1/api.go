package notifyv1api

import (
	"context"
	"strconv"

	notifyv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/notify/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	notifysvc "notify/internal/service/notify"
)

type ctxKeyUserID struct{}

func WithUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, ctxKeyUserID{}, id)
}

func ParseUserIDString(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func userIDFromCtx(ctx context.Context) (int64, error) {
	v := ctx.Value(ctxKeyUserID{})
	if v == nil {
		return 0, status.Error(codes.Unauthenticated, "missing user")
	}
	id, ok := v.(int64)
	if !ok || id == 0 {
		return 0, status.Error(codes.Unauthenticated, "invalid user")
	}
	return id, nil
}

type Implementation struct {
	notifyv1.UnimplementedNotifyServiceServer
	svc *notifysvc.Service
}

func New(svc *notifysvc.Service) *Implementation {
	return &Implementation{svc: svc}
}

func (i *Implementation) RegisterDevice(ctx context.Context, req *notifyv1.RegisterDeviceRequest) (*emptypb.Empty, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetPushToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "push_token required")
	}
	if err := i.svc.RegisterDevice(ctx, uid, req.GetPlatform(), req.GetPushToken()); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &emptypb.Empty{}, nil
}

func (i *Implementation) UnregisterDevice(ctx context.Context, req *notifyv1.UnregisterDeviceRequest) (*emptypb.Empty, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := i.svc.UnregisterDevice(ctx, uid, req.GetPushToken()); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &emptypb.Empty{}, nil
}
