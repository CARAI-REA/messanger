package wsv1

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gateway/internal/hub"
)

func TestBeginDrainAndWaitEmpty(t *testing.T) {
	h := NewHandler(hub.New(), nil, nil, nil, nil, false, 0, 0)
	if h.IsDraining() {
		t.Fatal("should not drain initially")
	}
	if !h.BeginDrain() {
		t.Fatal("BeginDrain should succeed once")
	}
	if !h.IsDraining() {
		t.Fatal("expected draining")
	}
	if h.BeginDrain() {
		t.Fatal("second BeginDrain should be false")
	}
	// no active conns → WaitEmpty returns immediately
	start := time.Now()
	h.WaitEmpty(2 * time.Second)
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("WaitEmpty with zero conns should return quickly")
	}
}

func TestHealthzReadyzMux(t *testing.T) {
	h := NewHandler(hub.New(), nil, nil, nil, nil, false, 0, 0)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if h.IsDraining() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != 200 {
		t.Fatalf("healthz=%d", rr.Code)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != 200 {
		t.Fatalf("readyz before drain=%d", rr.Code)
	}

	h.BeginDrain()
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != 503 {
		t.Fatalf("readyz draining=%d", rr.Code)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != 200 {
		t.Fatalf("healthz must stay 200 while draining, got %d", rr.Code)
	}
}
