package tokens

import (
	"context"
)

type JoinTokenGenerator interface {
	GenerateJoinToken(context.Context, JoinInfo) (string, error)
}

type JoinTokenVerifier interface {
	VerifyJoinToken(context.Context, string) (*JoinClaims, error)
}

type JoinTokenService interface {
	JoinTokenGenerator
	JoinTokenVerifier
}

type AccessTokenVerifier interface {
	VerifyAccessToken(context.Context, string) (*AccessClaims, error)
}
