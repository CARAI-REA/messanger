package outbox_test

import (
	"context"
	"testing"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"chat/internal/repository/outbox"
)

func TestOutboxSkipLockedStagedPublish(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	if !testutil.DockerAvailable(t) {
		t.Skip("docker unavailable")
	}

	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("chat"),
		postgres.WithUsername("chat"),
		postgres.WithPassword("chat"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	_, err = pool.Exec(ctx, `
CREATE TABLE outbox (
  id BIGSERIAL PRIMARY KEY,
  topic TEXT NOT NULL,
  partition_key TEXT NOT NULL,
  payload BYTEA NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  published_at TIMESTAMPTZ NULL,
  kafka_published_at TIMESTAMPTZ NULL,
  realtime_published_at TIMESTAMPTZ NULL
)`)
	if err != nil {
		t.Fatal(err)
	}

	repo := outbox.NewRepository(pool)
	if err := repo.Insert(ctx, "chat.events", "42", []byte(`{"t":1}`)); err != nil {
		t.Fatal(err)
	}

	rows, hold, err := outbox.BeginClaimHeld(repo, ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("claimed=%d want 1", len(rows))
	}
	if err := hold.MarkKafkaPublished(ctx, rows[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := hold.MarkRealtimePublished(ctx, rows[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := hold.MarkFullyPublished(ctx, rows[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := hold.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	rows2, hold2, err := outbox.BeginClaimHeld(repo, ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer hold2.Rollback(ctx)
	if len(rows2) != 0 {
		t.Fatalf("pending after publish: %d", len(rows2))
	}
}
