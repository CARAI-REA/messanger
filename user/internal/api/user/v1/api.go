package userv1api

import (
	"context"
	"strconv"
	"strings"

	userv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/user/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"user/internal/model"
	"user/internal/repository"
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
	id, err := i.svc.Create(ctx, model.UserInfo{
		Name:     req.UserInfo.Name,
		Email:    req.UserInfo.Email,
		Username: req.UserInfo.Username,
	}, req.Password, req.PasswordConfirm)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "already taken") {
			return nil, status.Error(codes.AlreadyExists, msg)
		}
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	return &userv1.CreateResponse{Id: id}, nil
}

func (i *Implementation) Get(ctx context.Context, req *userv1.GetRequest) (*userv1.GetResponse, error) {
	u, err := i.svc.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}
	return &userv1.GetResponse{User: toPublicProto(u)}, nil
}

func (i *Implementation) GetMe(ctx context.Context, _ *userv1.GetMeRequest) (*userv1.GetResponse, error) {
	id, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	u, err := i.svc.Get(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}
	return &userv1.GetResponse{User: toPrivateProto(u)}, nil
}

func (i *Implementation) GetByUsername(ctx context.Context, req *userv1.GetByUsernameRequest) (*userv1.GetResponse, error) {
	u, err := i.svc.GetByUsername(ctx, req.GetUsername())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}
	return &userv1.GetResponse{User: toPublicProto(u)}, nil
}

func (i *Implementation) SearchUsers(ctx context.Context, req *userv1.SearchUsersRequest) (*userv1.SearchUsersResponse, error) {
	users, err := i.svc.Search(ctx, req.GetQuery(), int(req.GetLimit()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	out := make([]*userv1.User, 0, len(users))
	for _, u := range users {
		out = append(out, toPublicProto(u))
	}
	return &userv1.SearchUsersResponse{Users: out}, nil
}

func (i *Implementation) Update(ctx context.Context, req *userv1.UpdateRequest) (*emptypb.Empty, error) {
	id, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	upd := repository.UserUpdate{}
	if req.Name != nil {
		v := req.Name.GetValue()
		upd.Name = &v
	}
	if req.Email != nil {
		v := req.Email.GetValue()
		upd.Email = &v
	}
	if req.Username != nil {
		v := req.Username.GetValue()
		upd.Username = &v
	}
	if req.AvatarFileId != nil {
		v := req.AvatarFileId.GetValue()
		upd.AvatarFileID = &v
	}
	if err := i.svc.Update(ctx, id, upd); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "already taken") {
			return nil, status.Error(codes.AlreadyExists, msg)
		}
		if strings.Contains(msg, "username") {
			return nil, status.Error(codes.InvalidArgument, msg)
		}
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

func (i *Implementation) Delete(ctx context.Context, _ *userv1.DeleteRequest) (*emptypb.Empty, error) {
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

func toPublicProto(u *model.User) *userv1.User {
	return &userv1.User{
		Id: u.ID,
		UserInfo: &userv1.UserInfo{
			Name:     u.Info.Name,
			Username: u.Info.Username,
			// email intentionally omitted for public views
		},
		AvatarFileId: u.AvatarFileID,
		CreatedAt:    timestamppb.New(u.CreatedAt),
		UpdatedAt:    timestamppb.New(u.UpdatedAt),
	}
}

func toPrivateProto(u *model.User) *userv1.User {
	p := toPublicProto(u)
	p.UserInfo.Email = u.Info.Email
	return p
}

// ParseUserIDString converts JWT user_id claim.
func ParseUserIDString(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
