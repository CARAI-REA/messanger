package outbox

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"user/internal/repository"
)

type repo struct {
	db *pgxpool.Pool
	sb sq.StatementBuilderType
}

func NewRepository(db *pgxpool.Pool) repository.OutboxRepository {
	return &repo{db: db, sb: sq.StatementBuilder.PlaceholderFormat(sq.Dollar)}
}

func (r *repo) Insert(ctx context.Context, topic, key string, payload []byte) error {
	q, args, err := r.sb.Insert("outbox").Columns("topic", "partition_key", "payload").Values(topic, key, payload).ToSql()
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, q, args...)
	return err
}

func (r *repo) ClaimPending(ctx context.Context, limit int) ([]repository.OutboxRow, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := claimRows(ctx, tx, limit)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return rows, nil
}

func claimRows(ctx context.Context, tx pgx.Tx, limit int) ([]repository.OutboxRow, error) {
	q := `
SELECT id, topic, partition_key, payload, kafka_published_at
FROM outbox
WHERE published_at IS NULL
ORDER BY id ASC
LIMIT $1
FOR UPDATE SKIP LOCKED`
	rows, err := tx.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []repository.OutboxRow
	for rows.Next() {
		var row repository.OutboxRow
		if err := rows.Scan(&row.ID, &row.Topic, &row.Key, &row.Payload, &row.KafkaPublishedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

type TxRepo struct {
	tx pgx.Tx
}

func (r *repo) BeginClaim(ctx context.Context, limit int) ([]repository.OutboxRow, *TxRepo, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	rows, err := claimRows(ctx, tx, limit)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, nil, err
	}
	return rows, &TxRepo{tx: tx}, nil
}

func (t *TxRepo) MarkKafkaPublished(ctx context.Context, id int64) error {
	_, err := t.tx.Exec(ctx, `UPDATE outbox SET kafka_published_at = NOW() WHERE id = $1 AND kafka_published_at IS NULL`, id)
	return err
}

func (t *TxRepo) MarkFullyPublished(ctx context.Context, id int64) error {
	_, err := t.tx.Exec(ctx, `
UPDATE outbox
SET published_at = NOW(),
    kafka_published_at = COALESCE(kafka_published_at, NOW())
WHERE id = $1 AND published_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("mark fully published: %w", err)
	}
	return nil
}

func (t *TxRepo) Commit(ctx context.Context) error   { return t.tx.Commit(ctx) }
func (t *TxRepo) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }

func (r *repo) MarkKafkaPublished(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE outbox SET kafka_published_at = NOW() WHERE id = $1 AND kafka_published_at IS NULL`, id)
	return err
}

func (r *repo) MarkFullyPublished(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `
UPDATE outbox
SET published_at = NOW(),
    kafka_published_at = COALESCE(kafka_published_at, NOW())
WHERE id = $1 AND published_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("mark fully published: %w", err)
	}
	return nil
}

func (r *repo) CountPending(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE published_at IS NULL`).Scan(&n)
	return n, err
}

func Concrete(r repository.OutboxRepository) *repo {
	if c, ok := r.(*repo); ok {
		return c
	}
	return nil
}
