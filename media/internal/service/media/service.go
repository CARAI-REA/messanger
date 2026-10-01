package media

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"media/internal/model"
	mediarepo "media/internal/repository/media"
	s3store "media/internal/s3"
)

type Service struct {
	repo *mediarepo.Repository
	s3   *s3store.Client
}

func NewService(repo *mediarepo.Repository, s3 *s3store.Client) *Service {
	return &Service{repo: repo, s3: s3}
}

func (s *Service) InitUpload(ctx context.Context, ownerID int64, filename, mime string, sizeBytes int64) (fileID, putURL string, err error) {
	if sizeBytes > 0 && sizeBytes > s.s3.MaxUploadBytes() {
		return "", "", fmt.Errorf("file too large")
	}
	if err := s.s3.ValidateMIME(mime); err != nil {
		return "", "", err
	}
	id := uuid.NewString()
	key := fmt.Sprintf("%d/%s/%s", ownerID, id, filename)
	f := &model.File{
		ID: id, OwnerID: ownerID, ObjectKey: key, Filename: filename,
		Mime: mime, SizeBytes: sizeBytes, Status: "pending",
	}
	if err := s.repo.Create(ctx, f); err != nil {
		return "", "", err
	}
	url, err := s.s3.PresignPut(ctx, key, mime)
	if err != nil {
		return "", "", err
	}
	return id, url, nil
}

func (s *Service) CompleteUpload(ctx context.Context, ownerID int64, fileID string) (*model.File, string, error) {
	f, err := s.repo.Get(ctx, fileID)
	if err != nil {
		return nil, "", err
	}
	if f.OwnerID != ownerID {
		return nil, "", fmt.Errorf("forbidden")
	}
	size, err := s.s3.Stat(ctx, f.ObjectKey)
	if err != nil {
		return nil, "", err
	}
	f, err = s.repo.MarkReady(ctx, fileID, size)
	if err != nil {
		return nil, "", err
	}
	url, err := s.s3.PresignGet(ctx, f.ObjectKey)
	if err != nil {
		return nil, "", err
	}
	return f, url, nil
}

func (s *Service) GetFile(ctx context.Context, fileID string) (*model.File, string, error) {
	f, err := s.repo.Get(ctx, fileID)
	if err != nil {
		return nil, "", err
	}
	if f.Status != "ready" {
		return nil, "", fmt.Errorf("file not ready")
	}
	url, err := s.s3.PresignGet(ctx, f.ObjectKey)
	if err != nil {
		return nil, "", err
	}
	return f, url, nil
}

func (s *Service) DeleteFile(ctx context.Context, ownerID int64, fileID string) error {
	f, err := s.repo.Delete(ctx, fileID, ownerID)
	if err != nil {
		return err
	}
	_ = s.s3.Remove(ctx, f.ObjectKey)
	return nil
}
