package wsv1

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/logger"
	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"gateway/internal/hub"
	"gateway/internal/metrics"
	"gateway/internal/presence"
)

type membershipLoader interface {
	ListChatIDs(ctx context.Context, userID int64) (map[int64]struct{}, error)
}

type Handler struct {
	hub             *hub.Hub
	presence        *presence.Store
	verifier        tokens.AccessTokenVerifier
	memberships     membershipLoader
	origins         map[string]struct{}
	allowQueryToken bool
	maxConnections  int64
	maxMsgPerSec    int
	activeConns     atomic.Int64
	draining        atomic.Bool
	upgrader        websocket.Upgrader
}

func NewHandler(
	h *hub.Hub,
	p *presence.Store,
	verifier tokens.AccessTokenVerifier,
	memberships membershipLoader,
	origins []string,
	allowQueryToken bool,
	maxConnections int,
	maxMsgPerSec int,
) *Handler {
	originSet := make(map[string]struct{}, len(origins))
	allowAll := false
	for _, o := range origins {
		if o == "*" {
			allowAll = true
		}
		originSet[o] = struct{}{}
	}
	handler := &Handler{
		hub:             h,
		presence:        p,
		verifier:        verifier,
		memberships:     memberships,
		origins:         originSet,
		allowQueryToken: allowQueryToken,
		maxConnections:  int64(maxConnections),
		maxMsgPerSec:    maxMsgPerSec,
	}
	handler.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			if allowAll || len(originSet) == 0 {
				return true
			}
			_, ok := originSet[r.Header.Get("Origin")]
			return ok
		},
	}
	return handler
}

func extractToken(r *http.Request, allowQueryToken bool) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if !allowQueryToken {
		return ""
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

func (h *Handler) IsDraining() bool {
	return h.draining.Load()
}

func (h *Handler) BeginDrain() bool {
	return h.draining.CompareAndSwap(false, true)
}

func (h *Handler) ActiveConnections() int64 {
	return h.activeConns.Load()
}

func (h *Handler) WaitEmpty(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if h.activeConns.Load() <= 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (h *Handler) tryAcquireConnection() bool {
	if h.maxConnections <= 0 {
		n := h.activeConns.Add(1)
		metrics.WSConnections.Set(float64(n))
		return true
	}
	for {
		current := h.activeConns.Load()
		if current >= h.maxConnections {
			return false
		}
		if h.activeConns.CompareAndSwap(current, current+1) {
			metrics.WSConnections.Set(float64(current + 1))
			return true
		}
	}
}

func (h *Handler) releaseConnection() {
	n := h.activeConns.Add(-1)
	metrics.WSConnections.Set(float64(n))
	metrics.DisconnectsTotal.Inc()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.IsDraining() {
		http.Error(w, "gateway draining", http.StatusServiceUnavailable)
		return
	}
	token := extractToken(r, h.allowQueryToken)
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}
	claims, err := h.verifier.VerifyAccessToken(r.Context(), token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	uid, err := strconv.ParseInt(claims.UserUUID, 10, 64)
	if err != nil || uid == 0 {
		http.Error(w, "invalid user_id", http.StatusUnauthorized)
		return
	}

	chatIDs := map[int64]struct{}{}
	if h.memberships != nil {
		chatIDs, err = h.memberships.ListChatIDs(r.Context(), uid)
		if err != nil {
			logger.Error(r.Context(), "load chat memberships", zap.Error(err), zap.Int64("user_id", uid))
			http.Error(w, "chat membership unavailable", http.StatusServiceUnavailable)
			return
		}
	}
	if !h.tryAcquireConnection() {
		http.Error(w, "too many websocket connections", http.StatusServiceUnavailable)
		return
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.releaseConnection()
		logger.Error(r.Context(), "ws upgrade", zap.Error(err))
		return
	}

	client := &hub.Client{
		UserID: uid,
		Conn:   conn,
		Send:   make(chan []byte, 64),
		Chats:  chatIDs,
	}
	h.hub.Register(client)
	_ = h.presence.SetOnline(uid)
	logger.Info(r.Context(), "ws connected", zap.Int64("user_id", uid), zap.Int("chats", len(chatIDs)))

	go h.writePump(client)
	go h.readPump(r.Context(), client)
}

func (h *Handler) writePump(c *hub.Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.Conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				return
			}
			metrics.MessagesTotal.WithLabelValues("out").Inc()
		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			_ = h.presence.Heartbeat(c.UserID)
		}
	}
}

type inbound struct {
	Type   string `json:"type"`
	ChatID int64  `json:"chat_id"`
}

func (h *Handler) readPump(ctx context.Context, c *hub.Client) {
	defer func() {
		h.hub.Unregister(c)
		_ = h.presence.SetOffline(c.UserID)
		_ = c.Conn.Close()
		h.releaseConnection()
	}()
	c.Conn.SetReadLimit(64 << 10)
	_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_ = h.presence.Heartbeat(c.UserID)
		return nil
	})
	limiter := newRateLimiter(h.maxMsgPerSec)
	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			return
		}
		metrics.MessagesTotal.WithLabelValues("in").Inc()
		if !limiter.Allow() {
			select {
			case c.Send <- []byte(`{"type":"error","code":"rate_limited","message":"rate limited"}`):
			default:
			}
			if limiter.Violations() >= 3 {
				_ = c.Conn.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "rate limited"),
					time.Now().Add(time.Second),
				)
				return
			}
			continue
		}
		var msg inbound
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "typing":
			if msg.ChatID > 0 {
				_ = h.presence.SetTyping(msg.ChatID, c.UserID)
				payload, _ := json.Marshal(map[string]any{
					"type":    "typing",
					"chat_id": msg.ChatID,
					"user_id": c.UserID,
				})
				h.hub.SendToChat(msg.ChatID, payload)
			}
		case "ping":
			_ = h.presence.Heartbeat(c.UserID)
			select {
			case c.Send <- []byte(`{"type":"pong"}`):
			default:
			}
		}
	}
}
