package tokens

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ServiceClaims identify internal callers (signaling, sfu).
type ServiceClaims struct {
	jwt.RegisteredClaims
	Service string `json:"svc"`
}

type ServiceTokenGenerator interface {
	GenerateServiceToken(ctx context.Context, service string, ttl time.Duration) (string, error)
}

type ServiceTokenVerifier interface {
	VerifyServiceToken(ctx context.Context, token string) (*ServiceClaims, error)
}

type ServiceTokenService interface {
	ServiceTokenGenerator
	ServiceTokenVerifier
}
