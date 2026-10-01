package kafka

import (
	"context"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/logger"
	"github.com/IBM/sarama"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"user/internal/repository"
	outboxrepo "user/internal/repository/outbox"
)

var (
	outboxPending = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "outbox_pending",
		Help:        "Number of unpublished outbox rows",
		ConstLabels: prometheus.Labels{"service": "user"},
	})
	outboxPublishErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "outbox_publish_errors_total",
		Help:        "Outbox publish errors by stage",
		ConstLabels: prometheus.Labels{"service": "user"},
	}, []string{"stage"})
)

func init() {
	prometheus.MustRegister(outboxPending, outboxPublishErrors)
}

type OutboxWorker struct {
	repo repository.OutboxRepository
	sync sarama.SyncProducer
}

func NewSyncProducerConfig() *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V3_6_0_0
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Retry.Max = 5
	cfg.Producer.Return.Successes = true
	return cfg
}

func NewOutboxWorker(repo repository.OutboxRepository, brokers []string) (*OutboxWorker, error) {
	sp, err := sarama.NewSyncProducer(brokers, NewSyncProducerConfig())
	if err != nil {
		return nil, err
	}
	return &OutboxWorker{repo: repo, sync: sp}, nil
}

func (w *OutboxWorker) Run(ctx context.Context) {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.flush(ctx)
		}
	}
}

func (w *OutboxWorker) flush(ctx context.Context) {
	if n, err := w.repo.CountPending(ctx); err == nil {
		outboxPending.Set(float64(n))
	}

	concrete := outboxrepo.Concrete(w.repo)
	if concrete == nil {
		logger.Error(ctx, "outbox concrete repo unavailable")
		return
	}
	rows, tx, err := concrete.BeginClaim(ctx, 100)
	if err != nil {
		logger.Error(ctx, "outbox claim", zap.Error(err))
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, row := range rows {
		if row.KafkaPublishedAt == nil {
			_, _, err := w.sync.SendMessage(&sarama.ProducerMessage{
				Topic: row.Topic,
				Key:   sarama.ByteEncoder(row.Key),
				Value: sarama.ByteEncoder(row.Payload),
			})
			if err != nil {
				outboxPublishErrors.WithLabelValues("kafka").Inc()
				logger.Error(ctx, "outbox kafka publish", zap.Error(err), zap.Int64("id", row.ID))
				break
			}
			if err := tx.MarkKafkaPublished(ctx, row.ID); err != nil {
				outboxPublishErrors.WithLabelValues("kafka_mark").Inc()
				logger.Error(ctx, "outbox mark kafka", zap.Error(err), zap.Int64("id", row.ID))
				break
			}
		}
		if err := tx.MarkFullyPublished(ctx, row.ID); err != nil {
			outboxPublishErrors.WithLabelValues("full_mark").Inc()
			logger.Error(ctx, "outbox mark full", zap.Error(err), zap.Int64("id", row.ID))
		}
	}

	if err := tx.Commit(ctx); err != nil {
		logger.Error(ctx, "outbox claim commit", zap.Error(err))
	}
}

func (w *OutboxWorker) Close() error {
	if w.sync != nil {
		return w.sync.Close()
	}
	return nil
}
