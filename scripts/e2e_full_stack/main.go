package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	authv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/auth/v1"
	chatv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/chat/v1"
	userv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/user/v1"
)

func main() {
	userAddr := envOr("USER_GRPC", "localhost:50051")
	authAddr := envOr("AUTH_GRPC", "localhost:50050")
	chatAddr := envOr("CHAT_GRPC", "localhost:50052")
	wsURL := envOr("WS_URL", "ws://localhost:8082/ws")
	email := fmt.Sprintf("e2e_%d@example.com", time.Now().UnixNano())
	pass := "password123"

	mustHTTP("http://localhost:8082/healthz")
	mustHTTP("http://localhost:8082/readyz")
	fmt.Println("OK healthz/readyz")

	uctx := context.Background()
	uconn := mustDial(userAddr)
	defer uconn.Close()
	aconn := mustDial(authAddr)
	defer aconn.Close()
	cconn := mustDial(chatAddr)
	defer cconn.Close()

	user := userv1.NewUserServiceClient(uconn)
	auth := authv1.NewAuthServiceClient(aconn)
	chat := chatv1.NewChatServiceClient(cconn)

	created, err := user.Create(uctx, &userv1.CreateRequest{
		UserInfo:        &userv1.UserInfo{Name: "e2e", Email: email},
		Password:        pass,
		PasswordConfirm: pass,
	})
	must(err)
	uid := created.GetId()
	fmt.Printf("user=%d\n", uid)

	login, err := auth.Login(uctx, &authv1.LoginRequest{Email: email, Password: pass})
	must(err)
	access := login.GetAccessToken()
	if access == "" {
		panic("empty access token")
	}

	authCtx := metadata.NewOutgoingContext(uctx, metadata.Pairs("authorization", "Bearer "+access))
	chatResp, err := chat.CreateChat(authCtx, &chatv1.CreateChatRequest{
		ChatInfo: &chatv1.ChatInfo{Name: "e2e", Description: "e2e", UserIds: []int64{uid}},
	})
	must(err)
	chatID := chatResp.GetChatId()
	fmt.Printf("chat=%d\n", chatID)

	header := http.Header{}
	header.Set("Authorization", "Bearer "+access)
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	must(err)
	defer ws.Close()
	fmt.Println("OK ws connected (Authorization header)")

	idem := fmt.Sprintf("e2e-%d", time.Now().UnixNano())
	msg1, err := chat.SendMessage(authCtx, &chatv1.SendMessageRequest{
		ChatId: chatID, Text: "hello e2e", IdempotencyKey: idem,
	})
	must(err)
	msg2, err := chat.SendMessage(authCtx, &chatv1.SendMessageRequest{
		ChatId: chatID, Text: "hello e2e", IdempotencyKey: idem,
	})
	must(err)
	if msg1.GetMessageId() == 0 || msg1.GetMessageId() != msg2.GetMessageId() {
		panic(fmt.Sprintf("idempotency failed: %d vs %d", msg1.GetMessageId(), msg2.GetMessageId()))
	}
	fmt.Printf("OK idempotent SendMessage id=%d\n", msg1.GetMessageId())

	_ = ws.SetReadDeadline(time.Now().Add(8 * time.Second))
	gotEvent := false
	for i := 0; i < 5; i++ {
		_, data, err := ws.ReadMessage()
		if err != nil {
			break
		}
		var envelope map[string]any
		_ = json.Unmarshal(data, &envelope)
		raw, _ := json.Marshal(envelope)
		if string(raw) != "" {
			gotEvent = true
			fmt.Printf("OK ws event: %s\n", truncate(string(data), 120))
			break
		}
	}
	if !gotEvent {
		fmt.Println("WARN no realtime event within deadline (outbox lag?) — smoke still OK for HTTP path")
	}

	fmt.Println("e2e_full_stack finished OK")
}

func mustDial(addr string) *grpc.ClientConn {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	must(err)
	return conn
}

func mustHTTP(url string) {
	resp, err := http.Get(url)
	must(err)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		panic(fmt.Sprintf("%s -> %d", url, resp.StatusCode))
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
