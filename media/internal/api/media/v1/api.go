package mediav1api

import (
	"context"
	"strconv"

	mediav1 "github.com/CARAI-REA/messanger/shared/pkg/proto/media/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	mediasvc "media/internal/service/media"
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
	mediav1.UnimplementedMediaServiceServer
	svc *mediasvc.Service
}

func New(svc *mediasvc.Service) *Implementation {
	return &Implementation{svc: svc}
}

func (i *Implementation) InitUpload(ctx context.Context, req *mediav1.InitUploadRequest) (*mediav1.InitUploadResponse, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	id, url, err := i.svc.InitUpload(ctx, uid, req.GetFilename(), req.GetMime(), req.GetSizeBytes())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	return &mediav1.InitUploadResponse{FileId: id, PutUrl: url}, nil
}

func (i *Implementation) CompleteUpload(ctx context.Context, req *mediav1.CompleteUploadRequest) (*mediav1.File, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	f, url, err := i.svc.CompleteUpload(ctx, uid, req.GetFileId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &mediav1.File{
		FileId: f.ID, OwnerId: f.OwnerID, Mime: f.Mime, SizeBytes: f.SizeBytes,
		Status: f.Status, GetUrl: url, CreatedAt: timestamppb.New(f.CreatedAt),
	}, nil
}

func (i *Implementation) GetFile(ctx context.Context, req *mediav1.GetFileRequest) (*mediav1.File, error) {
	if _, err := userIDFromCtx(ctx); err != nil {
		return nil, err
	}
	f, url, err := i.svc.GetFile(ctx, req.GetFileId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}
	return &mediav1.File{
		FileId: f.ID, OwnerId: f.OwnerID, Mime: f.Mime, SizeBytes: f.SizeBytes,
		Status: f.Status, GetUrl: url, CreatedAt: timestamppb.New(f.CreatedAt),
	}, nil
}

func (i *Implementation) DeleteFile(ctx context.Context, req *mediav1.DeleteFileRequest) (*mediav1.DeleteFileResponse, error) {
	uid, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := i.svc.DeleteFile(ctx, uid, req.GetFileId()); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &mediav1.DeleteFileResponse{}, nil
}
