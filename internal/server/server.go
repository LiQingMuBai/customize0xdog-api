package server

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"customize-teldog-api/internal/config"
	"customize-teldog-api/internal/teldog"
)

type Server struct {
	cfg    config.Config
	client *teldog.Client
	mux    *http.ServeMux
}

func New(cfg config.Config) (*Server, error) {
	c, err := teldog.NewClient(cfg.TeldogBaseURL, cfg.TeldogAPIKey, cfg.HTTPTimeout)
	if err != nil {
		return nil, err
	}

	s := &Server{
		cfg:    cfg,
		client: c,
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.logging(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)

	s.mux.HandleFunc("GET /api/teldog/balance", s.handleProxyGet("/openapi/agent/balance", nil))
	s.mux.HandleFunc("GET /api/teldog/countries", s.handleProxyGet("/openapi/agent/countries", nil))
	s.mux.HandleFunc("GET /api/teldog/operators", s.handleProxyGet("/openapi/agent/operators", []string{"country_iso"}))
	s.mux.HandleFunc("GET /api/teldog/products", s.handleProxyGet("/openapi/agent/products", []string{"country_iso"}))

	s.mux.HandleFunc("POST /api/teldog/orders", s.handleProxyPost("/openapi/agent/orders"))
	s.mux.HandleFunc("GET /api/teldog/orders/", s.handleGetOrderByAgentOrderID)

	s.mux.HandleFunc("POST /api/teldog/callback", s.handleCallback)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"service":   "customize-teldog-api",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleProxyGet(path string, requiredQueryKeys []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, k := range requiredQueryKeys {
			if strings.TrimSpace(r.URL.Query().Get(k)) == "" {
				writeAPIError(w, http.StatusUnprocessableEntity, 42201, "missing query: "+k)
				return
			}
		}

		q := copyQuery(r.URL.Query())
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		resp, err := s.client.Get(ctx, path, q)
		if err != nil && resp.Body == nil {
			writeAPIError(w, http.StatusBadGateway, 50000, "upstream request failed")
			return
		}
		writeUpstream(w, resp)
	}
}

func (s *Server) handleProxyPost(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, 42201, "invalid body")
			return
		}

		if len(strings.TrimSpace(string(b))) == 0 {
			writeAPIError(w, http.StatusUnprocessableEntity, 42201, "empty body")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		resp, callErr := s.client.PostJSON(ctx, path, b)
		if callErr != nil && resp.Body == nil {
			writeAPIError(w, http.StatusBadGateway, 50000, "upstream request failed")
			return
		}
		writeUpstream(w, resp)
	}
}

func (s *Server) handleGetOrderByAgentOrderID(w http.ResponseWriter, r *http.Request) {
	agentOrderID := strings.TrimPrefix(r.URL.Path, "/api/teldog/orders/")
	agentOrderID = strings.TrimSpace(agentOrderID)
	if agentOrderID == "" || strings.Contains(agentOrderID, "/") {
		writeAPIError(w, http.StatusUnprocessableEntity, 42201, "invalid agent_order_id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	resp, err := s.client.Get(ctx, "/openapi/agent/orders/"+url.PathEscape(agentOrderID), nil)
	if err != nil && resp.Body == nil {
		writeAPIError(w, http.StatusBadGateway, 50000, "upstream request failed")
		return
	}
	writeUpstream(w, resp)
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	ts := strings.TrimSpace(r.Header.Get("X-Callback-Timestamp"))
	sig := strings.TrimSpace(r.Header.Get("X-Callback-Signature"))
	if ts == "" || sig == "" {
		writeAPIError(w, http.StatusUnauthorized, 40101, "missing callback signature headers")
		return
	}

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, 42201, "invalid body")
		return
	}

	expect := callbackSignatureHex(s.cfg.TeldogAPIKey, ts, raw)
	if !secureEqualHex(sig, expect) {
		writeAPIError(w, http.StatusUnauthorized, 40101, "invalid callback signature")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func writeAPIError(w http.ResponseWriter, statusCode int, code int, message string) {
	writeJSON(w, statusCode, apiResponse{
		Code:    code,
		Message: message,
		Data:    map[string]any{},
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeUpstream(w http.ResponseWriter, resp teldog.Response) {
	ct := resp.ContentType
	if ct == "" {
		ct = "application/json"
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(resp.Body)
}

func copyQuery(q url.Values) url.Values {
	out := url.Values{}
	for k, vs := range q {
		out[k] = append([]string(nil), vs...)
	}
	return out
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &respWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s status=%d dur=%s", r.Method, r.URL.Path, rw.status, time.Since(start).String())
	})
}

type respWriter struct {
	http.ResponseWriter
	status int
}

func (w *respWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
