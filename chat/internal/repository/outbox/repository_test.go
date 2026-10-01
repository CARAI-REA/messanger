package outbox

import (
	"testing"
	"time"

	"chat/internal/repository"
)

func TestOutboxRowStages(t *testing.T) {
	now := time.Now()
	row := repository.OutboxRow{ID: 1, KafkaPublishedAt: &now}
	if row.KafkaPublishedAt == nil {
		t.Fatal("expected kafka stage set")
	}
	if row.RealtimePublishedAt != nil {
		t.Fatal("realtime should still be pending")
	}
}

func TestFullyPublishedRequiresBoth(t *testing.T) {
	now := time.Now()
	ready := func(r repository.OutboxRow) bool {
		return r.KafkaPublishedAt != nil && r.RealtimePublishedAt != nil
	}
	if ready(repository.OutboxRow{KafkaPublishedAt: &now}) {
		t.Fatal("should not be ready without realtime")
	}
	if !ready(repository.OutboxRow{KafkaPublishedAt: &now, RealtimePublishedAt: &now}) {
		t.Fatal("should be ready")
	}
}
