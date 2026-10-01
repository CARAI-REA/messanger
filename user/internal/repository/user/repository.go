package user

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
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

func (r *repo) Create(ctx context.Context, info *model.UserInfo, passwordHash string, createdAt time.Time, buildEvents func(id int64) ([]repository.OutboxEvent, error)) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	q, args, err := r.sb.Insert("users").
		Columns("name", "email", "password", "created_at", "updated_at").
		Values(info.Name, info.Email, passwordHash, createdAt, createdAt).
		Suffix("RETURNING id").ToSql()
	if err != nil {
		return 0, err
	}
	var id int64
	if err := tx.QueryRow(ctx, q, args...).Scan(&id); err != nil {
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

func (r *repo) Get(ctx context.Context, id int64) (*model.User, error) {
	q, args, err := r.sb.Select("id", "name", "email", "created_at", "updated_at").
		From("users").Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return nil, err
	}
	var u model.User
	err = r.db.QueryRow(ctx, q, args...).Scan(&u.ID, &u.Info.Name, &u.Info.Email, &u.CreatedAt, &u.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *repo) GetByEmail(ctx context.Context, email string) (*model.User, string, error) {
	q, args, err := r.sb.Select("id", "name", "email", "password", "created_at", "updated_at").
		From("users").Where(sq.Eq{"email": email}).ToSql()
	if err != nil {
		return nil, "", err
	}
	var u model.User
	var hash string
	err = r.db.QueryRow(ctx, q, args...).Scan(&u.ID, &u.Info.Name, &u.Info.Email, &hash, &u.CreatedAt, &u.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, "", fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, "", err
	}
	return &u, hash, nil
}

func (r *repo) Update(ctx context.Context, id int64, name, email *string) error {
	ub := r.sb.Update("users").Set("updated_at", time.Now()).Where(sq.Eq{"id": id})
	if name != nil {
		ub = ub.Set("name", *name)
	}
	if email != nil {
		ub = ub.Set("email", *email)
	}
	q, args, err := ub.ToSql()
	if err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, q, args...)
	if err != nil {
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
