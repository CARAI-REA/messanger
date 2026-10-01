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

type AccessJWTVerifierConfig interface {
	AuthTokenSecretKey() string
}

type AccessJWTVerifier struct {
	cfg AccessJWTVerifierConfig
}

func NewAccessJWTVerifier(cfg AccessJWTVerifierConfig) tokens.AccessTokenVerifier {
	return &AccessJWTVerifier{cfg: cfg}
}

func (v *AccessJWTVerifier) VerifyAccessToken(ctx context.Context, tokenStr string) (*tokens.AccessClaims, error) {
	if tokenStr == "" {
		return nil, fmt.Errorf("token string is empty")
	}

	secret := []byte(v.cfg.AuthTokenSecretKey())
	if len(secret) == 0 {
		return nil, fmt.Errorf("auth token secret key is empty")
	}

	t, err := jwt.ParseWithClaims(
		tokenStr,
		&tokens.AccessClaims{},
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				err := fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				logger.Info(ctx, "unexpected signing method for access token", zap.Error(err))
				return nil, err
			}
			return secret, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("invalid access token: %w", err)
	}

	if !t.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	claims, ok := t.Claims.(*tokens.AccessClaims)
	if !ok {
		return nil, fmt.Errorf("invalid access token claims type")
	}

	if claims.UserUUID == "" {
		return nil, fmt.Errorf("user_id claim is empty")
	}

	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
		return nil, fmt.Errorf("token has expired")
	}

	return claims, nil
}
