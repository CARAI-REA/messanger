package jwt

import (
	"context"
	"fmt"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
	"github.com/golang-jwt/jwt/v5"
)

type ServiceJWTConfig interface {
	ServiceTokenSecretKey() string
}

type ServiceJWTService struct {
	cfg ServiceJWTConfig
}

func NewServiceJWTService(cfg ServiceJWTConfig) tokens.ServiceTokenService {
	return &ServiceJWTService{cfg: cfg}
}

func (s *ServiceJWTService) GenerateServiceToken(_ context.Context, service string, ttl time.Duration) (string, error) {
	secret := []byte(s.cfg.ServiceTokenSecretKey())
	if len(secret) == 0 {
		return "", fmt.Errorf("service token secret is empty")
	}
	if service == "" {
		return "", fmt.Errorf("service name is empty")
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	now := time.Now()
	claims := tokens.ServiceClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   service,
		},
		Service: service,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(secret)
}

func (s *ServiceJWTService) VerifyServiceToken(_ context.Context, tokenStr string) (*tokens.ServiceClaims, error) {
	secret := []byte(s.cfg.ServiceTokenSecretKey())
	if len(secret) == 0 {
		return nil, fmt.Errorf("service token secret is empty")
	}
	claims := &tokens.ServiceClaims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil || !tok.Valid {
		return nil, fmt.Errorf("invalid service token: %w", err)
	}
	if claims.Service == "" {
		return nil, fmt.Errorf("service claim missing")
	}
	return claims, nil
}
