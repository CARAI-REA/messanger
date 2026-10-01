package jwt

import (
	"context"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
)

// Конфиг для join-токенов
type JoinJWTConfig interface {
	JoinTokenSecretKey() string
	JoinTokenExpiration() time.Duration
}

type JoinJWTService struct {
	cfg       JoinJWTConfig
	generator tokens.JoinTokenGenerator
	verifier  tokens.JoinTokenVerifier
}

func NewJoinJWTService(cfg JoinJWTConfig) tokens.JoinTokenService {
	return &JoinJWTService{
		cfg:       cfg,
		generator: NewJoinJWTGenerator(cfg),
		verifier:  NewJoinJWTVerifier(cfg),
	}
}

func (s *JoinJWTService) GenerateJoinToken(ctx context.Context, info tokens.JoinInfo) (string, error) {
	return s.generator.GenerateJoinToken(ctx, info)
}

func (s *JoinJWTService) VerifyJoinToken(ctx context.Context, token string) (*tokens.JoinClaims, error) {
	return s.verifier.VerifyJoinToken(ctx, token)
}
