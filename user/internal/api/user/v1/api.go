package userv1api

import (
	"context"
	"strconv"

	userv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/user/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"user/internal/model"
	"user/internal/service"
)

type Implementation struct {
	userv1.UnimplementedUserServiceServer
	svc service.UserService
}

func NewImplementation(svc service.UserService) *Implementation {
	return &Implementation{svc: svc}
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

type ctxKeyUserID struct{}

func WithUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, ctxKeyUserID{}, id)
}

func (i *Implementation) Create(ctx context.Context, req *userv1.CreateRequest) (*userv1.CreateResponse, error) {
	if req.GetUserInfo() == nil {
		return nil, status.Error(codes.InvalidArgument, "user_info required")
	}
	id, err := i.svc.Create(ctx, model.UserInfo{Name: req.UserInfo.Name, Email: req.UserInfo.Email}, req.Password, req.PasswordConfirm)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	return &userv1.CreateResponse{Id: id}, nil
}

func (i *Implementation) Get(ctx context.Context, req *userv1.GetRequest) (*userv1.GetResponse, error) {
	u, err := i.svc.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}
	return &userv1.GetResponse{User: toProto(u)}, nil
}

func (i *Implementation) Update(ctx context.Context, req *userv1.UpdateRequest) (*emptypb.Empty, error) {
	id, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	var name, email *string
	if req.Name != nil {
		v := req.Name.GetValue()
		name = &v
	}
	if req.Email != nil {
		v := req.Email.GetValue()
		email = &v
	}
	if err := i.svc.Update(ctx, id, name, email); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &emptypb.Empty{}, nil
}

func (i *Implementation) UpdatePassword(ctx context.Context, req *userv1.UpdatePasswordRequest) (*emptypb.Empty, error) {
	id, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := i.svc.UpdatePassword(ctx, id, req.Password, req.PasswordConfirm, ""); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	return &emptypb.Empty{}, nil
}

func (i *Implementation) Delete(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	id, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := i.svc.Delete(ctx, id); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &emptypb.Empty{}, nil
}

func (i *Implementation) ValidateCredentials(ctx context.Context, req *userv1.ValidateCredentialsRequest) (*userv1.ValidateCredentialsResponse, error) {
	ok, id, err := i.svc.ValidateCredentials(ctx, req.Email, req.Password)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &userv1.ValidateCredentialsResponse{Valid: ok, UserId: id}, nil
}

func toProto(u *model.User) *userv1.User {
	return &userv1.User{
		Id: u.ID,
		UserInfo: &userv1.UserInfo{Name: u.Info.Name, Email: u.Info.Email},
		CreatedAt: timestamppb.New(u.CreatedAt),
		UpdatedAt: timestamppb.New(u.UpdatedAt),
	}
}

// ParseUserIDString converts JWT user_id claim.
func ParseUserIDString(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
