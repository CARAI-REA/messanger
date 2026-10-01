package chatgrpc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/grpctls"
	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
	chatv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/chat/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Client struct {
	conn        *grpc.ClientConn
	client      chatv1.ChatServiceClient
	tokenGen    tokens.ServiceTokenGenerator
	serviceName string
}

type DialConfig struct {
	Address    string
	UseTLS     bool
	CAFile     string
	ServerName string
}

func New(cfg DialConfig, tokenGen tokens.ServiceTokenGenerator, serviceName string) (*Client, error) {
	dialOpt, err := grpctls.DialOption(cfg.UseTLS, cfg.CAFile, cfg.ServerName)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(
		cfg.Address,
		dialOpt,
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:        conn,
		client:      chatv1.NewChatServiceClient(conn),
		tokenGen:    tokenGen,
		serviceName: serviceName,
	}, nil
}

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) ListChatIDs(ctx context.Context, userID int64) (map[int64]struct{}, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	ctx, err := c.withServiceAuth(ctx, userID)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.ListChatIDs(ctx, &chatv1.ListChatIDsRequest{})
	if err != nil {
		return nil, fmt.Errorf("list chat ids: %w", err)
	}

	chatIDs := make(map[int64]struct{}, len(resp.GetChatIds()))
	for _, chatID := range resp.GetChatIds() {
		if chatID <= 0 {
			continue
		}
		chatIDs[chatID] = struct{}{}
	}
	return chatIDs, nil
}

func (c *Client) withServiceAuth(ctx context.Context, userID int64) (context.Context, error) {
	if c.tokenGen == nil {
		return ctx, nil
	}

	token, err := c.tokenGen.GenerateServiceToken(ctx, c.serviceName, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("service token: %w", err)
	}

	md := metadata.Pairs(
		"authorization", "Bearer "+token,
		"x-user-id", strconv.FormatInt(userID, 10),
	)
	return metadata.NewOutgoingContext(ctx, md), nil
}
