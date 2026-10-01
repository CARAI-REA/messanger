package app

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/CARAI-REA/messanger/platform/pkg/closer"
	"github.com/CARAI-REA/messanger/platform/pkg/logger"
	"github.com/CARAI-REA/messanger/platform/pkg/prodguard"
	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
	jwtTokens "github.com/CARAI-REA/messanger/platform/pkg/tokens/jwt"
	eventsv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/events/v1"
	"github.com/gomodule/redigo/redis"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	wsv1 "gateway/internal/api/ws/v1"
	"gateway/internal/chatgrpc"
	"gateway/internal/config"
	"gateway/internal/hub"
	"gateway/internal/presence"
	"gateway/internal/rediswire"
)

type diContainer struct {
	pool           *redis.Pool
	hub            *hub.Hub
	presence       *presence.Store
	accessVerifier tokens.AccessTokenVerifier
	serviceTokens  tokens.ServiceTokenService
	chatClient     *chatgrpc.Client
	wsHandler      *wsv1.Handler
}

func NewDiContainer() *diContainer {
	return &diContainer{}
}

func (d *diContainer) Redis() *redis.Pool {
	if d.pool != nil {
		return d.pool
	}
	cfg := config.AppConfig().Redis
	d.pool = rediswire.NewPool(cfg.Address(), cfg.Password(), cfg.MaxIdle(), cfg.IdleTimeout(), cfg.ConnTimeout())
	closer.AddNamed("redis", func(context.Context) error { return d.pool.Close() })
	return d.pool
}

func (d *diContainer) Hub() *hub.Hub {
	if d.hub != nil {
		return d.hub
	}
	d.hub = hub.New()
	return d.hub
}

func (d *diContainer) Presence() *presence.Store {
	if d.presence != nil {
		return d.presence
	}
	gw := config.AppConfig().Gateway
	d.presence = presence.New(d.Redis(), gw.PresenceTTL(), gw.TypingTTL())
	return d.presence
}

func (d *diContainer) AccessVerifier() tokens.AccessTokenVerifier {
	if d.accessVerifier != nil {
		return d.accessVerifier
	}
	d.accessVerifier = jwtTokens.NewAccessJWTVerifier(config.AppConfig().JWT)
	return d.accessVerifier
}

func (d *diContainer) ServiceTokens() tokens.ServiceTokenService {
	if d.serviceTokens != nil {
		return d.serviceTokens
	}
	d.serviceTokens = jwtTokens.NewServiceJWTService(config.AppConfig().JWT)
	return d.serviceTokens
}

func (d *diContainer) ChatClient() *chatgrpc.Client {
	if d.chatClient != nil {
		return d.chatClient
	}
	client, err := chatgrpc.New(
		chatgrpc.DialConfig{
			Address:    config.AppConfig().Gateway.ChatGRPCAddress(),
			UseTLS:     config.AppConfig().Gateway.ChatGRPCTLS(),
			CAFile:     config.AppConfig().Gateway.ChatGRPCCAFile(),
			ServerName: config.AppConfig().Gateway.ChatGRPCServerName(),
		},
		d.ServiceTokens(),
		"gateway",
	)
	if err != nil {
		panic(err)
	}
	closer.AddNamed("chat grpc client", func(context.Context) error { return client.Close() })
	d.chatClient = client
	return d.chatClient
}

func (d *diContainer) WSHandler() *wsv1.Handler {
	if d.wsHandler != nil {
		return d.wsHandler
	}
	cfg := config.AppConfig()
	allowQueryToken := cfg.Gateway.AllowQueryToken() && !prodguard.IsProduction(cfg.App.Env())
	d.wsHandler = wsv1.NewHandler(
		d.Hub(),
		d.Presence(),
		d.AccessVerifier(),
		d.ChatClient(),
		cfg.Gateway.AllowedOrigins(),
		allowQueryToken,
		cfg.Gateway.MaxWSConnections(),
		cfg.Gateway.MaxMsgPerSec(),
	)
	return d.wsHandler
}

func (d *diContainer) StartRealtimeSubscriber(ctx context.Context) {
	channel := config.AppConfig().Gateway.RealtimeChannel()
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			if err := d.subscribeLoop(ctx, channel); err != nil {
				logger.Error(ctx, "realtime subscribe", zap.Error(err))
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()
}

func (d *diContainer) subscribeLoop(ctx context.Context, channel string) error {
	conn := d.Redis().Get()
	defer conn.Close()
	psc := redis.PubSubConn{Conn: conn}
	if err := psc.Subscribe(channel); err != nil {
		return err
	}
	logger.Info(ctx, "subscribed to realtime channel", zap.String("channel", channel))
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			d.forwardRealtime(v.Data)
		case redis.Subscription:
			continue
		case error:
			return v
		}
		if ctx.Err() != nil {
			_ = psc.Unsubscribe()
			return ctx.Err()
		}
	}
}

func (d *diContainer) forwardRealtime(data []byte) {
	var ev eventsv1.ChatRealtimeEvent
	if err := proto.Unmarshal(data, &ev); err != nil {
		var payload struct {
			ChatID int64 `json:"chat_id"`
		}
		if json.Unmarshal(data, &payload) == nil && payload.ChatID > 0 {
			d.Hub().SendToChat(payload.ChatID, data)
			return
		}
		d.Hub().Broadcast(data)
		return
	}

	// Update live membership before fanout so new members receive this event.
	if mc := ev.GetMemberChanged(); mc != nil {
		switch mc.GetAction() {
		case "added", "created", "role_updated":
			d.Hub().AddChat(mc.GetUserId(), ev.GetChatId())
		case "removed":
			// Deliver leave notice to the removed user, then drop membership.
			payload, err := json.Marshal(map[string]any{
				"type":     "chat.realtime",
				"event_id": ev.EventId,
				"chat_id":  ev.ChatId,
				"actor_id": ev.ActorId,
				"payload":  base64.StdEncoding.EncodeToString(data),
			})
			if err == nil {
				d.Hub().SendToUser(mc.GetUserId(), payload)
			}
			d.Hub().RemoveChat(mc.GetUserId(), ev.GetChatId())
		}
	}

	payload, err := json.Marshal(map[string]any{
		"type":     "chat.realtime",
		"event_id": ev.EventId,
		"chat_id":  ev.ChatId,
		"actor_id": ev.ActorId,
		"payload":  base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		return
	}
	if ev.GetChatId() > 0 {
		d.Hub().SendToChat(ev.GetChatId(), payload)
		return
	}
	d.Hub().Broadcast(payload)
}
