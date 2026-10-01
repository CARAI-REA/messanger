package jwt

import (
	"context"
	"fmt"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/logger"
	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type JoinJWTGeneratorConfig interface {
	JoinTokenSecretKey() string
	JoinTokenExpiration() time.Duration
}

type JoinJWTGenerator struct {
	cfg JoinJWTGeneratorConfig
}

func NewJoinJWTGenerator(cfg JoinJWTGeneratorConfig) tokens.JoinTokenGenerator {
	return &JoinJWTGenerator{cfg: cfg}
}

func (g *JoinJWTGenerator) GenerateJoinToken(ctx context.Context, info tokens.JoinInfo) (string, error) {
	secret := []byte(g.cfg.JoinTokenSecretKey())
	if len(secret) == 0 {
		return "", fmt.Errorf("join token secret key is empty")
	}

	now := time.Now()

	claims := tokens.JoinClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(g.cfg.JoinTokenExpiration())),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
		UserUUID: info.GetUserUUID(),
		RoomUUID: info.GetRoomUUID(),
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := t.SignedString(secret)
	if err != nil {
		logger.Error(ctx, "failed to sign join token", zap.Error(err))
		return "", fmt.Errorf("failed to sign join token: %w", err)
	}

	return signed, nil
}
