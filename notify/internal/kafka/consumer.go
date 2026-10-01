package kafka

import (
	"context"
	"fmt"

	"github.com/CARAI-REA/messanger/platform/pkg/logger"
	eventsv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/events/v1"
	"github.com/IBM/sarama"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	notifysvc "notify/internal/service/notify"
)

type Consumer struct {
	group  sarama.ConsumerGroup
	topics []string
	svc    *notifysvc.Service
}

func NewConsumer(brokers []string, groupID string, topics []string, svc *notifysvc.Service) (*Consumer, error) {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V3_6_0_0
	cfg.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRange()
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	g, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, err
	}
	return &Consumer{group: g, topics: topics, svc: svc}, nil
}

func (c *Consumer) Run(ctx context.Context) {
	handler := &handler{svc: c.svc}
	for {
		if err := c.group.Consume(ctx, c.topics, handler); err != nil {
			logger.Error(ctx, "notify kafka consume", zap.Error(err))
		}
		if ctx.Err() != nil {
			return
		}
	}
}

func (c *Consumer) Close() error { return c.group.Close() }

type handler struct {
	svc *notifysvc.Service
}

func (h *handler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *handler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *handler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		var ev eventsv1.ChatRealtimeEvent
		if err := proto.Unmarshal(msg.Value, &ev); err != nil {
			session.MarkMessage(msg, "")
			continue
		}
		ctx := session.Context()
		switch p := ev.Payload.(type) {
		case *eventsv1.ChatRealtimeEvent_MessageCreated:
			title := "New message"
			body := p.MessageCreated.Text
			if len(body) > 120 {
				body = body[:120]
			}
			_ = h.svc.NotifyAll(ctx, title, fmt.Sprintf("chat %d: %s", ev.ChatId, body))
		}
		session.MarkMessage(msg, "")
	}
	return nil
}
