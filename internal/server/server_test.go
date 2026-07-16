package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"customize-teldog-api/internal/config"
)

func TestProxyBalance(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi/agent/balance" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "k" {
			t.Fatalf("missing api key header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"","data":{"balance":1}}`))
	}))
	t.Cleanup(upstream.Close)

	s, err := New(config.Config{
		ListenAddr:    ":0",
		TeldogBaseURL: upstream.URL,
		TeldogAPIKey:  "k",
		HTTPTimeout:   time.Second,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/teldog/balance", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Content-Type") == "" {
		t.Fatalf("missing content-type")
	}
	if rr.Body.String() != `{"code":0,"message":"","data":{"balance":1}}` {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func TestHealth(t *testing.T) {
	s, err := New(config.Config{
		ListenAddr:    ":0",
		TeldogBaseURL: "https://example.com",
		TeldogAPIKey:  "k",
		HTTPTimeout:   time.Second,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/health", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type=%q", got)
	}

	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status=%q", body["status"])
	}
	if body["service"] != "customize-teldog-api" {
		t.Fatalf("service=%q", body["service"])
	}
	if body["timestamp"] == "" {
		t.Fatal("timestamp is empty")
	}
}

func TestCallbackSignature(t *testing.T) {
	s, err := New(config.Config{
		ListenAddr:    ":0",
		TeldogBaseURL: "https://example.com",
		TeldogAPIKey:  "k",
		HTTPTimeout:   time.Second,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	ts := "1773741600"
	body := []byte(`{"agent_order_id":"A","status":"success"}`)
	sig := callbackSignatureHex("k", ts, body)

	req := httptest.NewRequest(http.MethodPost, "http://example.com/api/teldog/callback", bytes.NewReader(body))
	req.Header.Set("X-Callback-Timestamp", ts)
	req.Header.Set("X-Callback-Signature", sig)

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}
