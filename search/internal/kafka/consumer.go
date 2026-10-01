package kafka

import (
	"context"
	"fmt"

	"github.com/CARAI-REA/messanger/platform/pkg/logger"
	eventsv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/events/v1"
	"github.com/IBM/sarama"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	osclient "search/internal/opensearch"
)

var (
	indexErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "search_index_errors_total",
		Help: "Failed search index operations",
	})
	messagesConsumed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "search_messages_consumed_total",
		Help: "Search kafka messages consumed",
	})
)

func init() {
	prometheus.MustRegister(indexErrors, messagesConsumed)
}

type Consumer struct {
	group  sarama.ConsumerGroup
	topics []string
	index  *osclient.Client
}

func NewConsumer(brokers []string, groupID string, topics []string, index *osclient.Client) (*Consumer, error) {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V3_6_0_0
	cfg.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRange()
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	cfg.Consumer.Offsets.AutoCommit.Enable = false
	g, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, err
	}
	return &Consumer{group: g, topics: topics, index: index}, nil
}

func (c *Consumer) Run(ctx context.Context) {
	handler := &handler{index: c.index}
	for {
		if err := c.group.Consume(ctx, c.topics, handler); err != nil {
			logger.Error(ctx, "search kafka consume", zap.Error(err))
		}
		if ctx.Err() != nil {
			return
		}
	}
}

func (c *Consumer) Close() error {
	return c.group.Close()
}

type handler struct {
	index *osclient.Client
}

func (h *handler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *handler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *handler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		messagesConsumed.Inc()
		var ev eventsv1.ChatRealtimeEvent
		if err := proto.Unmarshal(msg.Value, &ev); err != nil {
			logger.Error(session.Context(), "search bad event", zap.Error(err))
			session.MarkMessage(msg, "")
			session.Commit()
			continue
		}
		ctx := session.Context()
		if err := h.indexEvent(ctx, &ev); err != nil {
			indexErrors.Inc()
			logger.Error(ctx, "search index failed", zap.Error(err), zap.Int64("chat_id", ev.ChatId))
			return fmt.Errorf("index: %w", err)
		}
		session.MarkMessage(msg, "")
		session.Commit()
	}
	return nil
}

func (h *handler) indexEvent(ctx context.Context, ev *eventsv1.ChatRealtimeEvent) error {
	switch p := ev.Payload.(type) {
	case *eventsv1.ChatRealtimeEvent_MessageCreated:
		m := p.MessageCreated
		return h.index.IndexMessage(ctx, osclient.MessageDoc{
			MessageID: m.MessageId,
			ChatID:    ev.ChatId,
			SenderID:  m.SenderId,
			Text:      m.Text,
		})
	case *eventsv1.ChatRealtimeEvent_MessageEdited:
		return h.index.UpdateMessageText(ctx, p.MessageEdited.MessageId, p.MessageEdited.Text)
	case *eventsv1.ChatRealtimeEvent_MessageDeleted:
		return h.index.DeleteMessage(ctx, p.MessageDeleted.MessageId)
	default:
		return nil
	}
}
