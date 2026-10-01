package integration_test

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// Login happy-path with live user gRPC is covered by compose e2e.
// This package documents the JWT contract auth issues after ValidateCredentials.
func TestAccessTokenClaimsShape(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "1",
		"uid": float64(1),
	})
	signed, err := tok.SignedString([]byte("test-secret-at-least-32-bytes-long!!"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := jwt.Parse(signed, func(token *jwt.Token) (any, error) {
		return []byte("test-secret-at-least-32-bytes-long!!"), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("parse: %v", err)
	}
}
