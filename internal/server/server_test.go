package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"customize-teldog-api/internal/config"
)

func TestHealth(t *testing.T) {
	s := newTestServer(t, "http://localhost:9999", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "http://localhost:9999/health", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	logHTTPResponse(t, rr)

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

func TestHealthz(t *testing.T) {
	s := newTestServer(t, "http://localhost:9999", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "http://localhost:9999/healthz", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	logHTTPResponse(t, rr)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestProxyBalance(t *testing.T) {
	s := newTestServer(t, "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi/agent/balance" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "k" {
			t.Fatalf("missing api key header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"","data":{"balance":1}}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "http://localhost:9999/api/teldog/balance", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	logHTTPResponse(t, rr)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != `{"code":0,"message":"","data":{"balance":1}}` {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func TestOperatorsRequiresCountryISO(t *testing.T) {
	s := newTestServer(t, "http://localhost:9999", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "http://localhost:9999/api/teldog/operators", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	assertAPIError(t, rr, http.StatusUnprocessableEntity, 42201, "missing query: country_iso")
}

func TestProductsForwardsQuery(t *testing.T) {
	s := newTestServer(t, "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi/agent/products" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("country_iso"); got != "US" {
			t.Fatalf("country_iso=%q", got)
		}
		if got := r.URL.Query().Get("operator_code"); got != "att" {
			t.Fatalf("operator_code=%q", got)
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"","data":[]}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "http://localhost:9999/api/teldog/products?country_iso=US&operator_code=att", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	logHTTPResponse(t, rr)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCreateOrderRejectsEmptyBody(t *testing.T) {
	s := newTestServer(t, "http://localhost:9999", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodPost, "http://localhost:9999/api/teldog/orders", strings.NewReader("   "))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	assertAPIError(t, rr, http.StatusUnprocessableEntity, 42201, "empty body")
}

func TestCreateOrderForwardsBody(t *testing.T) {
	payload := `{"agent_order_id":"A-1","product_code":"us-att-10","phone":"123456"}`
	s := newTestServer(t, "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi/agent/orders" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
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
			t.Fatalf("unexpected body: %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"message":"","data":{"status":"processing"}}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "http://localhost:9999/api/teldog/orders", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	logHTTPResponse(t, rr)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGetOrderByAgentOrderID(t *testing.T) {
	s := newTestServer(t, "", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi/agent/orders/AGT-1001" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"","data":{"status":"success"}}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "http://localhost:9999/api/teldog/orders/AGT-1001", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	logHTTPResponse(t, rr)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGetOrderRejectsInvalidID(t *testing.T) {
	s := newTestServer(t, "http://localhost:9999", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "http://localhost:9999/api/teldog/orders/a/b", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	assertAPIError(t, rr, http.StatusUnprocessableEntity, 42201, "invalid agent_order_id")
}

func TestUpstreamConnectionFailureReturnsBadGateway(t *testing.T) {
	s, err := New(config.Config{
		ListenAddr:    ":0",
		TeldogBaseURL: "http://127.0.0.1:1",
		TeldogAPIKey:  "k",
		HTTPTimeout:   200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost:9999/api/teldog/balance", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	assertAPIError(t, rr, http.StatusBadGateway, 50000, "upstream request failed")
}

func TestCallbackSignature(t *testing.T) {
	s := newTestServer(t, "http://localhost:9999", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	ts := "1773741600"
	body := []byte(`{"agent_order_id":"A","status":"success"}`)
	sig := callbackSignatureHex("k", ts, body)

	req := httptest.NewRequest(http.MethodPost, "http://localhost:9999/api/teldog/callback", bytes.NewReader(body))
	req.Header.Set("X-Callback-Timestamp", ts)
	req.Header.Set("X-Callback-Signature", sig)

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	logHTTPResponse(t, rr)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCallbackSignatureInvalid(t *testing.T) {
	s := newTestServer(t, "http://localhost:9999", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodPost, "http://localhost:9999/api/teldog/callback", strings.NewReader(`{"agent_order_id":"A"}`))
	req.Header.Set("X-Callback-Timestamp", "1773741600")
	req.Header.Set("X-Callback-Signature", "deadbeef")

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	assertAPIError(t, rr, http.StatusUnauthorized, 40101, "invalid callback signature")
}

func TestCallbackSignatureMissingHeaders(t *testing.T) {
	s := newTestServer(t, "http://localhost:9999", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodPost, "http://localhost:9999/api/teldog/callback", strings.NewReader(`{"agent_order_id":"A"}`))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	assertAPIError(t, rr, http.StatusUnauthorized, 40101, "missing callback signature headers")
}

func TestCopyQuery(t *testing.T) {
	src := url.Values{
		"country_iso":   {"US"},
		"operator_code": {"att"},
	}

	dst := copyQuery(src)
	dst.Set("country_iso", "CA")

	if got := src.Get("country_iso"); got != "US" {
		t.Fatalf("source mutated: %q", got)
	}
}

func newTestServer(t *testing.T, baseURL string, handler http.Handler) *Server {
	t.Helper()

	if baseURL == "" {
		upstream := httptest.NewServer(handler)
		t.Cleanup(upstream.Close)
		baseURL = upstream.URL
	}

	s, err := New(config.Config{
		ListenAddr:    ":0",
		TeldogBaseURL: baseURL,
		TeldogAPIKey:  "k",
		HTTPTimeout:   time.Second,
		UIUsername:    "u",
		UIPassword:    "p",
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return s
}

func assertAPIError(t *testing.T, rr *httptest.ResponseRecorder, wantStatus int, wantCode int, wantMessage string) {
	t.Helper()
	logHTTPResponse(t, rr)

	if rr.Code != wantStatus {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}

	var body apiResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != wantCode {
		t.Fatalf("code=%d", body.Code)
	}
	if body.Message != wantMessage {
		t.Fatalf("message=%q", body.Message)
	}
}

func TestUIOrderPage(t *testing.T) {
	s := newTestServer(t, "http://localhost:9999", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "http://localhost:9999/ui/order", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Location"); got != "/ui/login" {
		t.Fatalf("location=%q", got)
	}
}

func TestUILoginPage(t *testing.T) {
	s := newTestServer(t, "http://localhost:9999", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "http://localhost:9999/ui/login", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content-type=%q", got)
	}
	if !strings.Contains(rr.Body.String(), "下单测试 — 登录") {
		t.Fatalf("unexpected body")
	}
	if !strings.Contains(rr.Body.String(), "captcha_code") {
		t.Fatalf("captcha field missing")
	}
}

func TestUILoginSubmitOK(t *testing.T) {
	s := newTestServer(t, "http://localhost:9999", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	rrLogin := httptest.NewRecorder()
	reqLogin := httptest.NewRequest(http.MethodGet, "http://localhost:9999/ui/login", nil)
	s.Handler().ServeHTTP(rrLogin, reqLogin)
	if rrLogin.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", rrLogin.Code, rrLogin.Body.String())
	}

	html := rrLogin.Body.String()
	code := extractInputValue(html, `name="captcha_code" value="`)
	token := extractInputValue(html, `name="captcha_token" value="`)
	if code == "" || token == "" {
		t.Fatalf("failed to extract captcha")
	}

	form := url.Values{}
	form.Set("username", "u")
	form.Set("password", "p")
	form.Set("captcha", code)
	form.Set("captcha_code", code)
	form.Set("captcha_token", token)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://localhost:9999/ui/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Location"); got != "/ui/order" {
		t.Fatalf("location=%q", got)
	}
	setCookie := rr.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "ui_session=") {
		t.Fatalf("missing session cookie")
	}

	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("no cookies")
	}

	rrOrder := httptest.NewRecorder()
	reqOrder := httptest.NewRequest(http.MethodGet, "http://localhost:9999/ui/order", nil)
	for _, c := range cookies {
		reqOrder.AddCookie(c)
	}
	s.Handler().ServeHTTP(rrOrder, reqOrder)

	if rrOrder.Code != http.StatusOK {
		t.Fatalf("order status=%d body=%s", rrOrder.Code, rrOrder.Body.String())
	}
	if !strings.Contains(rrOrder.Body.String(), "下单测试") {
		t.Fatalf("unexpected order body")
	}
}

func extractInputValue(html string, prefix string) string {
	_, after, ok := strings.Cut(html, prefix)
	if !ok {
		return ""
	}
	v, _, ok := strings.Cut(after, `"`)
	if !ok {
		return ""
	}
	return v
}

func logHTTPResponse(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	t.Logf("response status=%d body=%s", rr.Code, strings.TrimSpace(rr.Body.String()))
}
