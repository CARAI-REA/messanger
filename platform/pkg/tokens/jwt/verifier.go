package jwt

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/logger"
	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type JoinJWTVerifierConfig interface {
	JoinTokenSecretKey() string
}

type JoinJWTVerifier struct {
	cfg JoinJWTVerifierConfig
}

func NewJoinJWTVerifier(cfg JoinJWTVerifierConfig) tokens.JoinTokenVerifier {
	return &JoinJWTVerifier{cfg: cfg}
}

func (v *JoinJWTVerifier) VerifyJoinToken(ctx context.Context, tokenStr string) (*tokens.JoinClaims, error) {
	if tokenStr == "" {
		return nil, fmt.Errorf("token string is empty")
	}

	secret := []byte(v.cfg.JoinTokenSecretKey())

	t, err := jwt.ParseWithClaims(
		tokenStr,
		&tokens.JoinClaims{},
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				err := fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				logger.Info(ctx, "unexpected signing method for join token", zap.Error(err))
				return nil, err
			}
			return secret, nil
		},
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: %v", tokens.ErrTokenExpired, err)
		}
		return nil, fmt.Errorf("invalid join token: %w", err)
	}

	if !t.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	claims, ok := t.Claims.(*tokens.JoinClaims)
	if !ok {
		return nil, fmt.Errorf("invalid join token claims type")
	}

	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
		return nil, tokens.ErrTokenExpired
	}

	return claims, nil
}
