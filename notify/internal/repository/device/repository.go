package device

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Device struct {
	UserID    int64
	Platform  string
	PushToken string
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Register(ctx context.Context, userID int64, platform, token string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO devices (user_id, platform, push_token)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, push_token) DO UPDATE SET platform = EXCLUDED.platform, updated_at = NOW()`,
		userID, platform, token)
	return err
}

func (r *Repository) Unregister(ctx context.Context, userID int64, token string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM devices WHERE user_id = $1 AND push_token = $2`, userID, token)
	return err
}

func (r *Repository) ListByUser(ctx context.Context, userID int64) ([]Device, error) {
	rows, err := r.db.Query(ctx, `SELECT user_id, platform, push_token FROM devices WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.UserID, &d.Platform, &d.PushToken); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repository) ListAllTokens(ctx context.Context) ([]Device, error) {
	rows, err := r.db.Query(ctx, `SELECT user_id, platform, push_token FROM devices`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.UserID, &d.Platform, &d.PushToken); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
