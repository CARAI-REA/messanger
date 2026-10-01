package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
	userv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/user/v1"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	authrepo "auth/internal/repository/auth"
)

type Service struct {
	repo          *authrepo.Repository
	user          userv1.UserServiceClient
	serviceTok    tokens.ServiceTokenService
	accessSecret  string
	refreshSecret string
	accessTTL     time.Duration
	refreshTTL    time.Duration
	conn          *grpc.ClientConn
}

type UserDialConfig struct {
	Address    string
	TLS        bool
	CAFile     string
	ServerName string
}

func NewService(
	repo *authrepo.Repository,
	userDial UserDialConfig,
	serviceTok tokens.ServiceTokenService,
	accessSecret, refreshSecret string,
	accessTTL, refreshTTL time.Duration,
) (*Service, error) {
	creds := insecure.NewCredentials()
	if userDial.TLS {
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: userDial.ServerName}
		if userDial.CAFile != "" {
			pem, err := os.ReadFile(userDial.CAFile)
			if err != nil {
				return nil, fmt.Errorf("read USER_GRPC_CA_FILE: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("invalid USER_GRPC_CA_FILE")
			}
			tlsCfg.RootCAs = pool
		}
		creds = credentials.NewTLS(tlsCfg)
	}
	conn, err := grpc.NewClient(userDial.Address, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	return &Service{
		repo: repo, user: userv1.NewUserServiceClient(conn), serviceTok: serviceTok,
		accessSecret: accessSecret, refreshSecret: refreshSecret,
		accessTTL: accessTTL, refreshTTL: refreshTTL, conn: conn,
	}, nil
}

func (s *Service) Close() error { return s.conn.Close() }

func (s *Service) withServiceAuth(ctx context.Context) (context.Context, error) {
	tok, err := s.serviceTok.GenerateServiceToken(ctx, "auth", time.Hour)
	if err != nil {
		return nil, err
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tok), nil
}

func (s *Service) Login(ctx context.Context, email, password string) (refresh, access string, err error) {
	locked, err := s.repo.IsLocked(email)
	if err != nil {
		return "", "", err
	}
	if locked {
		return "", "", fmt.Errorf("too many login attempts")
	}

	ctx2, err := s.withServiceAuth(ctx)
	if err != nil {
		return "", "", err
	}
	resp, err := s.user.ValidateCredentials(ctx2, &userv1.ValidateCredentialsRequest{Email: email, Password: password})
	if err != nil {
		return "", "", err
	}
	if !resp.Valid {
		_, _ = s.repo.IncrLoginAttempts(email)
		return "", "", fmt.Errorf("invalid credentials")
	}
	_ = s.repo.ResetLoginAttempts(email)
	refresh, err = s.issueRefresh(resp.UserId)
	if err != nil {
		return "", "", err
	}
	access, err = s.issueAccess(resp.UserId)
	return refresh, access, err
}

func (s *Service) issueAccess(userID int64) (string, error) {
	now := time.Now()
	claims := tokens.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		UserUUID: strconv.FormatInt(userID, 10),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(s.accessSecret))
}

type refreshClaims struct {
	jwt.RegisteredClaims
	UserID int64  `json:"uid"`
	JTI    string `json:"jti"`
}

func (s *Service) issueRefresh(userID int64) (string, error) {
	jti := uuid.NewString()
	now := time.Now()
	claims := refreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jti,
		},
		UserID: userID,
		JTI:    jti,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString([]byte(s.refreshSecret))
	if err != nil {
		return "", err
	}
	if err := s.repo.StoreRefresh(jti, userID); err != nil {
		return "", err
	}
	return signed, nil
}

func (s *Service) parseRefresh(tokenStr string) (*refreshClaims, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &refreshClaims{}, func(t *jwt.Token) (any, error) {
		return []byte(s.refreshSecret), nil
	})
	if err != nil || !t.Valid {
		return nil, fmt.Errorf("invalid refresh token")
	}
	claims, ok := t.Claims.(*refreshClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}
	uid, err := s.repo.GetRefreshUserID(claims.JTI)
	if err != nil || uid != claims.UserID {
		return nil, fmt.Errorf("refresh revoked")
	}
	return claims, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (string, error) {
	claims, err := s.parseRefresh(refreshToken)
	if err != nil {
		return "", err
	}
	locked, err := s.repo.IsRefreshLocked(claims.UserID)
	if err != nil {
		return "", err
	}
	if locked {
		return "", fmt.Errorf("too many refresh attempts")
	}
	_, _ = s.repo.IncrRefreshAttempts(claims.UserID)

	newJTI := uuid.NewString()
	now := time.Now()
	newClaims := refreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        newJTI,
		},
		UserID: claims.UserID,
		JTI:    newJTI,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, newClaims)
	signed, err := t.SignedString([]byte(s.refreshSecret))
	if err != nil {
		return "", err
	}
	ok, err := s.repo.RotateRefresh(claims.JTI, newJTI, claims.UserID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("refresh revoked")
	}
	return signed, nil
}

func (s *Service) Access(ctx context.Context, refreshToken string) (string, error) {
	claims, err := s.parseRefresh(refreshToken)
	if err != nil {
		return "", err
	}
	return s.issueAccess(claims.UserID)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	claims, err := s.parseRefresh(refreshToken)
	if err != nil {
		return err
	}
	return s.repo.RevokeRefresh(claims.JTI, claims.UserID)
}

func (s *Service) ValidateAccess(ctx context.Context, accessToken string) error {
	_, err := jwt.ParseWithClaims(accessToken, &tokens.AccessClaims{}, func(t *jwt.Token) (any, error) {
		return []byte(s.accessSecret), nil
	})
	return err
}
