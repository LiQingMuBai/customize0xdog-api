package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const liveProxyBaseURL = "http://127.0.0.1:9999"

func TestLiveProxyHealth(t *testing.T) {
	runLiveProxyTest(t, http.MethodGet, liveProxyBaseURL+"/health", "", http.StatusOK)
}

func TestLiveProxyHealthz(t *testing.T) {
	runLiveProxyTest(t, http.MethodGet, liveProxyBaseURL+"/healthz", "", http.StatusNoContent)
}

func TestLiveProxyBalance(t *testing.T) {
	runLiveProxyTest(t, http.MethodGet, liveProxyBaseURL+"/api/teldog/balance", "", http.StatusOK)
}

func TestLiveProxyCountries(t *testing.T) {
	runLiveProxyTest(t, http.MethodGet, liveProxyBaseURL+"/api/teldog/countries", "", http.StatusOK)
}

func TestLiveProxyOperators(t *testing.T) {
	runLiveProxyTest(t, http.MethodGet, liveProxyBaseURL+"/api/teldog/operators?country_iso=US", "", http.StatusOK)
}

func TestLiveProxyProducts(t *testing.T) {
	runLiveProxyTest(t, http.MethodGet, liveProxyBaseURL+"/api/teldog/products?country_iso=US", "", http.StatusOK)
}

func TestLiveProxyMalaysiaProducts(t *testing.T) {
	status, raw := runLiveProxyJSONTest(t, http.MethodGet, liveProxyBaseURL+"/api/teldog/products?country_iso=MY", "")
	if status != http.StatusOK {
		t.Fatalf("unexpected status=%d body=%s", status, raw)
	}

	var resp liveAPIResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, string(raw))
	}
	if resp.Code != 0 {
		t.Fatalf("unexpected code=%d message=%s body=%s", resp.Code, resp.Message, string(raw))
	}
}

func TestLiveProxyMalaysiaCreateOrder(t *testing.T) {
	if os.Getenv("RUN_LIVE_PROXY_ORDER_TEST") != "1" {
		t.Skip("set RUN_LIVE_PROXY_ORDER_TEST=1 to run live malaysia order test")
	}

	productCode := strings.TrimSpace(os.Getenv("LIVE_MY_PRODUCT_CODE"))
	if productCode == "" {
		t.Skip("set LIVE_MY_PRODUCT_CODE to a valid Malaysia product_code before running order test")
	}

	agentOrderID := strings.TrimSpace(os.Getenv("LIVE_MY_AGENT_ORDER_ID"))
	if agentOrderID == "" {
		agentOrderID = fmt.Sprintf("AUTO-MY-%d", time.Now().Unix())
	}

	payload := map[string]string{
		"agent_order_id": agentOrderID,
		"product_code":   productCode,
	}

	if phone := strings.TrimSpace(os.Getenv("LIVE_MY_PHONE")); phone != "" {
		payload["phone"] = phone
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	status, raw := runLiveProxyJSONTest(t, http.MethodPost, liveProxyBaseURL+"/api/teldog/orders", string(body))
	if status != http.StatusOK {
		t.Fatalf("unexpected status=%d body=%s", status, raw)
	}

	var resp liveAPIResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, string(raw))
	}
	if resp.Code != 0 {
		t.Fatalf("unexpected code=%d message=%s body=%s", resp.Code, resp.Message, string(raw))
	}
}

func runLiveProxyTest(t *testing.T, method, reqURL, body string, wantStatus int) {
	t.Helper()

	status, raw := runLiveProxyJSONTest(t, method, reqURL, body)
	if status != wantStatus {
		t.Fatalf("unexpected status=%d body=%s", status, string(raw))
	}
}

func runLiveProxyJSONTest(t *testing.T, method, reqURL, body string) (int, []byte) {
	t.Helper()

	if os.Getenv("RUN_LIVE_PROXY_TEST") != "1" {
		t.Skip("set RUN_LIVE_PROXY_TEST=1 to run live proxy test against :9999")
	}
	if testing.Short() {
		t.Skip("skip live proxy test in short mode")
	}

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, reqURL, bodyReader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	t.Logf("live response status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	return resp.StatusCode, raw
}

type liveAPIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}
