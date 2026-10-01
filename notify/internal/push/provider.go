package push

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus"

	devicerepo "notify/internal/repository/device"
)

var (
	pushSent = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "notify_push_sent_total",
		Help: "Push send attempts by provider and result",
	}, []string{"provider", "result"})
)

func init() {
	prometheus.MustRegister(pushSent)
}

// ErrInvalidToken indicates the device token should be unregistered.
var ErrInvalidToken = errors.New("invalid push token")

type Provider interface {
	Send(ctx context.Context, device devicerepo.Device, title, body string) error
}

type MockProvider struct{}

func NewMock() *MockProvider { return &MockProvider{} }

func (m *MockProvider) Send(ctx context.Context, device devicerepo.Device, title, body string) error {
	_ = ctx
	_ = device
	_ = title
	_ = body
	pushSent.WithLabelValues("mock", "success").Inc()
	return nil
}

type serviceAccount struct {
	Type        string `json:"type"`
	ProjectID   string `json:"project_id"`
	PrivateKey  string `json:"private_key"`
	ClientEmail string `json:"client_email"`
	TokenURI    string `json:"token_uri"`
}

// FCMProvider sends via FCM HTTP v1 using a service-account JSON file.
type FCMProvider struct {
	projectID string
	email     string
	key       *rsa.PrivateKey
	tokenURI  string
	http      *http.Client

	mu    sync.Mutex
	token string
	exp   time.Time
}

func NewFCM(credentialsFile string) (*FCMProvider, error) {
	if credentialsFile == "" {
		return nil, fmt.Errorf("FCM_CREDENTIALS_FILE is required when PUSH_PROVIDER=fcm")
	}
	raw, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("FCM credentials: %w", err)
	}
	var sa serviceAccount
	if err := json.Unmarshal(raw, &sa); err != nil {
		return nil, fmt.Errorf("parse FCM credentials: %w", err)
	}
	if sa.ProjectID == "" || sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, fmt.Errorf("FCM credentials missing project_id/client_email/private_key")
	}
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return nil, fmt.Errorf("FCM private key pem decode failed")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse FCM private key: %w", err)
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("FCM private key is not RSA")
	}
	tokenURI := sa.TokenURI
	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token"
	}
	return &FCMProvider{
		projectID: sa.ProjectID,
		email:     sa.ClientEmail,
		key:       rsaKey,
		tokenURI:  tokenURI,
		http:      &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (f *FCMProvider) accessToken(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.token != "" && time.Now().Before(f.exp.Add(-60*time.Second)) {
		return f.token, nil
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   f.email,
		"scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud":   f.tokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := tok.SignedString(f.key)
	if err != nil {
		return "", err
	}
	form := "grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Ajwt-bearer&assertion=" + signed
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.tokenURI, strings.NewReader(form))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := f.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("oauth token %d: %s", resp.StatusCode, string(body))
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	f.token = out.AccessToken
	if out.ExpiresIn <= 0 {
		out.ExpiresIn = 3600
	}
	f.exp = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second)
	return f.token, nil
}

func (f *FCMProvider) Send(ctx context.Context, device devicerepo.Device, title, body string) error {
	token, err := f.accessToken(ctx)
	if err != nil {
		pushSent.WithLabelValues("fcm", "auth_error").Inc()
		return err
	}
	payload := map[string]any{
		"message": map[string]any{
			"token": device.PushToken,
			"notification": map[string]string{
				"title": title,
				"body":  body,
			},
		},
	}
	raw, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", f.projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.http.Do(req)
	if err != nil {
		pushSent.WithLabelValues("fcm", "transport_error").Inc()
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		pushSent.WithLabelValues("fcm", "success").Inc()
		return nil
	}
	msg := string(respBody)
	if resp.StatusCode == http.StatusNotFound ||
		strings.Contains(msg, "UNREGISTERED") ||
		strings.Contains(msg, "INVALID_ARGUMENT") ||
		strings.Contains(msg, "registration-token-not-registered") {
		pushSent.WithLabelValues("fcm", "invalid_token").Inc()
		return fmt.Errorf("%w: %s", ErrInvalidToken, msg)
	}
	pushSent.WithLabelValues("fcm", "error").Inc()
	return fmt.Errorf("fcm send %d: %s", resp.StatusCode, msg)
}

func New(provider, credentialsFile string) (Provider, error) {
	switch provider {
	case "fcm":
		return NewFCM(credentialsFile)
	default:
		return NewMock(), nil
	}
}
