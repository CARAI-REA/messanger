package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/testutil"
	"github.com/gomodule/redigo/redis"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"gateway/internal/hub"
)

func TestRedisPubSubMembershipFanout(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	if !testutil.DockerAvailable(t) {
		t.Skip("docker unavailable")
	}

	ctx := context.Background()
	container, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	endpoint, err := container.Endpoint(ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	pool := &redis.Pool{
		MaxIdle: 2,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", endpoint, redis.DialConnectTimeout(3*time.Second))
		},
	}
	t.Cleanup(func() { _ = pool.Close() })

	h := hub.New()
	c := &hub.Client{UserID: 9, Send: make(chan []byte, 4), Chats: map[int64]struct{}{}}
	h.Register(c)

	channel := "chat:realtime:test"
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn := pool.Get()
		defer conn.Close()
		psc := redis.PubSubConn{Conn: conn}
		if err := psc.Subscribe(channel); err != nil {
			t.Errorf("subscribe: %v", err)
			return
		}
		for {
			switch v := psc.Receive().(type) {
			case redis.Message:
				var evt struct {
					Type   string `json:"type"`
					ChatID int64  `json:"chat_id"`
					UserID int64  `json:"user_id"`
				}
				if err := json.Unmarshal(v.Data, &evt); err != nil {
					t.Errorf("unmarshal: %v", err)
					return
				}
				if evt.Type == "member_changed" && evt.UserID == 9 {
					h.AddChat(evt.UserID, evt.ChatID)
				}
				h.SendToChat(evt.ChatID, v.Data)
				return
			case error:
				t.Errorf("pubsub: %v", v)
				return
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
	pub := pool.Get()
	payload, _ := json.Marshal(map[string]any{
		"type": "member_changed", "chat_id": 77, "user_id": 9,
	})
	if _, err := pub.Do("PUBLISH", channel, payload); err != nil {
		t.Fatal(err)
	}
	pub.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for pubsub")
	}

	select {
	case got := <-c.Send:
		if string(got) != string(payload) {
			t.Fatalf("got %s", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("member did not receive fanout after hot AddChat")
	}
}
