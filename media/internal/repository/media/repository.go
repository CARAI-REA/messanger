package media

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"media/internal/model"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, f *model.File) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO files (id, owner_id, object_key, filename, mime, size_bytes, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8)`,
		f.ID, f.OwnerID, f.ObjectKey, f.Filename, f.Mime, f.SizeBytes, f.Status, time.Now().UTC())
	return err
}

func (r *Repository) Get(ctx context.Context, id string) (*model.File, error) {
	var f model.File
	err := r.db.QueryRow(ctx, `
		SELECT id, owner_id, object_key, filename, mime, size_bytes, status, created_at
		FROM files WHERE id = $1`, id,
	).Scan(&f.ID, &f.OwnerID, &f.ObjectKey, &f.Filename, &f.Mime, &f.SizeBytes, &f.Status, &f.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("file not found")
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *Repository) MarkReady(ctx context.Context, id string, size int64) (*model.File, error) {
	var f model.File
	err := r.db.QueryRow(ctx, `
		UPDATE files SET status = 'ready', size_bytes = COALESCE(NULLIF($2,0), size_bytes), updated_at = NOW()
		WHERE id = $1 RETURNING id, owner_id, object_key, filename, mime, size_bytes, status, created_at`,
		id, size,
	).Scan(&f.ID, &f.OwnerID, &f.ObjectKey, &f.Filename, &f.Mime, &f.SizeBytes, &f.Status, &f.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("file not found")
	}
	return &f, err
}

func (r *Repository) Delete(ctx context.Context, id string, ownerID int64) (*model.File, error) {
	var f model.File
	err := r.db.QueryRow(ctx, `
		DELETE FROM files WHERE id = $1 AND owner_id = $2
		RETURNING id, owner_id, object_key, filename, mime, size_bytes, status, created_at`,
		id, ownerID,
	).Scan(&f.ID, &f.OwnerID, &f.ObjectKey, &f.Filename, &f.Mime, &f.SizeBytes, &f.Status, &f.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("file not found")
	}
	return &f, err
}
