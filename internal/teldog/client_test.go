package teldog

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestClientGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method=%s", r.Method)
		}
		if r.URL.Path != "/openapi/agent/products" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if got := r.URL.Query().Get("country_iso"); got != "US" {
			t.Fatalf("country_iso=%q", got)
		}
		if got := r.Header.Get("X-API-Key"); got != "k" {
			t.Fatalf("api-key=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL, "k", time.Second)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	resp, err := client.Get(context.Background(), "/openapi/agent/products", url.Values{"country_iso": {"US"}})
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if string(resp.Body) != `{"code":0}` {
		t.Fatalf("body=%s", string(resp.Body))
	}
}

func TestClientPostJSON(t *testing.T) {
	payload := `{"agent_order_id":"A-1"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content-type=%q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(body) != payload {
			t.Fatalf("body=%s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL, "k", time.Second)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	resp, err := client.PostJSON(context.Background(), "/openapi/agent/orders", []byte(payload))
	if err != nil {
		t.Fatalf("post: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestClientReturnsResponseOnUpstreamServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":50000}`))
	}))
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL, "k", time.Second)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	resp, err := client.Get(context.Background(), "/openapi/agent/balance", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if string(resp.Body) != `{"code":50000}` {
		t.Fatalf("body=%s", string(resp.Body))
	}
}

func TestNewClientRejectsInvalidBaseURL(t *testing.T) {
	if _, err := NewClient("://bad-url", "k", time.Second); err == nil {
		t.Fatal("expected error")
	}
}
