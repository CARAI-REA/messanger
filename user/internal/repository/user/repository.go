package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"user/internal/model"
	"user/internal/repository"
)

type repo struct {
	db *pgxpool.Pool
	sb sq.StatementBuilderType
}

func NewRepository(db *pgxpool.Pool) repository.UserRepository {
	return &repo{db: db, sb: sq.StatementBuilder.PlaceholderFormat(sq.Dollar)}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (r *repo) Create(ctx context.Context, info *model.UserInfo, passwordHash string, createdAt time.Time, buildEvents func(id int64) ([]repository.OutboxEvent, error)) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	q, args, err := r.sb.Insert("users").
		Columns("name", "email", "username", "password", "created_at", "updated_at").
		Values(info.Name, info.Email, strings.ToLower(info.Username), passwordHash, createdAt, createdAt).
		Suffix("RETURNING id").ToSql()
	if err != nil {
		return 0, err
	}
	var id int64
	if err := tx.QueryRow(ctx, q, args...).Scan(&id); err != nil {
		if isUniqueViolation(err) {
			return 0, fmt.Errorf("email or username already taken")
		}
		return 0, fmt.Errorf("create user: %w", err)
	}
	if buildEvents != nil {
		events, err := buildEvents(id)
		if err != nil {
			return 0, err
		}
		for _, ev := range events {
			if _, err := tx.Exec(ctx,
				`INSERT INTO outbox (topic, partition_key, payload) VALUES ($1, $2, $3)`,
				ev.Topic, ev.Key, ev.Payload,
			); err != nil {
				return 0, fmt.Errorf("outbox insert: %w", err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return id, nil
}

func scanUser(row pgx.Row) (*model.User, error) {
	var u model.User
	var avatar *string
	err := row.Scan(&u.ID, &u.Info.Name, &u.Info.Email, &u.Info.Username, &avatar, &u.CreatedAt, &u.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}
	if avatar != nil {
		u.AvatarFileID = *avatar
	}
	return &u, nil
}

func (r *repo) Get(ctx context.Context, id int64) (*model.User, error) {
	q, args, err := r.sb.Select("id", "name", "email", "username", "avatar_file_id", "created_at", "updated_at").
		From("users").Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return nil, err
	}
	return scanUser(r.db.QueryRow(ctx, q, args...))
}

func (r *repo) GetByEmail(ctx context.Context, email string) (*model.User, string, error) {
	q, args, err := r.sb.Select("id", "name", "email", "username", "avatar_file_id", "password", "created_at", "updated_at").
		From("users").Where(sq.Eq{"email": email}).ToSql()
	if err != nil {
		return nil, "", err
	}
	var u model.User
	var avatar *string
	var hash string
	err = r.db.QueryRow(ctx, q, args...).Scan(&u.ID, &u.Info.Name, &u.Info.Email, &u.Info.Username, &avatar, &hash, &u.CreatedAt, &u.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, "", fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, "", err
	}
	if avatar != nil {
		u.AvatarFileID = *avatar
	}
	return &u, hash, nil
}

func (r *repo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	q, args, err := r.sb.Select("id", "name", "email", "username", "avatar_file_id", "created_at", "updated_at").
		From("users").Where(sq.Eq{"username": strings.ToLower(username)}).ToSql()
	if err != nil {
		return nil, err
	}
	return scanUser(r.db.QueryRow(ctx, q, args...))
}

func (r *repo) Search(ctx context.Context, query string, limit int) ([]*model.User, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	q = strings.TrimPrefix(q, "@")
	pattern := "%" + strings.ToLower(q) + "%"
	sql := `SELECT id, name, email, username, avatar_file_id, created_at, updated_at
		FROM users
		WHERE username ILIKE $1 OR name ILIKE $1
		ORDER BY CASE WHEN lower(username) = lower($2) THEN 0 ELSE 1 END, username
		LIMIT $3`
	rows, err := r.db.Query(ctx, sql, pattern, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.User
	for rows.Next() {
		var u model.User
		var avatar *string
		if err := rows.Scan(&u.ID, &u.Info.Name, &u.Info.Email, &u.Info.Username, &avatar, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		if avatar != nil {
			u.AvatarFileID = *avatar
		}
		out = append(out, &u)
	}
	return out, rows.Err()
}

func (r *repo) Update(ctx context.Context, id int64, upd repository.UserUpdate) error {
	ub := r.sb.Update("users").Set("updated_at", time.Now()).Where(sq.Eq{"id": id})
	if upd.Name != nil {
		ub = ub.Set("name", *upd.Name)
	}
	if upd.Email != nil {
		ub = ub.Set("email", *upd.Email)
	}
	if upd.Username != nil {
		ub = ub.Set("username", strings.ToLower(*upd.Username))
	}
	if upd.AvatarFileID != nil {
		ub = ub.Set("avatar_file_id", *upd.AvatarFileID)
	}
	q, args, err := ub.ToSql()
	if err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, q, args...)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("email or username already taken")
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *repo) UpdatePassword(ctx context.Context, id int64, passwordHash string, ip string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q, args, err := r.sb.Update("users").Set("password", passwordHash).Set("updated_at", time.Now()).Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, q, args...); err != nil {
		return err
	}
	lq, largs, err := r.sb.Insert("password_change_logs").Columns("user_id", "changed_at", "ip").Values(id, time.Now(), ip).ToSql()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, lq, largs...); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *repo) Delete(ctx context.Context, id int64, buildEvents func() ([]repository.OutboxEvent, error)) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q, args, err := r.sb.Delete("users").Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	if buildEvents != nil {
		events, err := buildEvents()
		if err != nil {
			return err
		}
		for _, ev := range events {
			if _, err := tx.Exec(ctx,
				`INSERT INTO outbox (topic, partition_key, payload) VALUES ($1, $2, $3)`,
				ev.Topic, ev.Key, ev.Payload,
			); err != nil {
				return fmt.Errorf("outbox insert: %w", err)
			}
		}
	}
	return tx.Commit(ctx)
}
