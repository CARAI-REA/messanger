package push

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	devicerepo "notify/internal/repository/device"
)

func writeTempSA(t *testing.T, tokenURI string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	sa := map[string]string{
		"type":         "service_account",
		"project_id":   "test-project",
		"private_key":  string(pemKey),
		"client_email": "fcm@test.iam.gserviceaccount.com",
		"token_uri":    tokenURI,
	}
	raw, _ := json.Marshal(sa)
	dir := t.TempDir()
	path := filepath.Join(dir, "sa.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMockProviderSend(t *testing.T) {
	p := NewMock()
	if err := p.Send(context.Background(), devicerepo.Device{UserID: 1, PushToken: "t"}, "hi", "body"); err != nil {
		t.Fatal(err)
	}
}

func TestFCMInvalidToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "atok", "expires_in": 3600})
	})
	mux.HandleFunc("/v1/projects/test-project/messages:send", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"status":"NOT_FOUND","details":[{"errorCode":"UNREGISTERED"}]}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	path := writeTempSA(t, srv.URL+"/token")
	p, err := NewFCM(path)
	if err != nil {
		t.Fatal(err)
	}
	// Point FCM HTTP client through test server by overriding project send URL via Transport redirect.
	p.http = srv.Client()
	p.tokenURI = srv.URL + "/token"
	// Monkey-patch by wrapping RoundTripper to rewrite FCM host.
	orig := p.http.Transport
	if orig == nil {
		orig = http.DefaultTransport
	}
	p.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "fcm.googleapis.com" {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
		}
		return orig.RoundTrip(req)
	})}

	err = p.Send(context.Background(), devicerepo.Device{UserID: 1, PushToken: "bad"}, "t", "b")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}

func TestFCMSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "atok", "expires_in": 3600})
	})
	mux.HandleFunc("/v1/projects/test-project/messages:send", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"projects/test/messages/1"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	path := writeTempSA(t, srv.URL+"/token")
	p, err := NewFCM(path)
	if err != nil {
		t.Fatal(err)
	}
	p.tokenURI = srv.URL + "/token"
	orig := http.DefaultTransport
	p.http = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "oauth2.googleapis.com" || req.URL.Path == "/token" || req.Host == srv.Listener.Addr().String() {
			return orig.RoundTrip(req)
		}
		if req.URL.Host == "fcm.googleapis.com" {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return orig.RoundTrip(req)
		}
		return orig.RoundTrip(req)
	})}

	if err := p.Send(context.Background(), devicerepo.Device{UserID: 1, PushToken: "good"}, "t", "b"); err != nil {
		t.Fatal(err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
