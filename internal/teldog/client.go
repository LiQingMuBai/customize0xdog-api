package teldog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL *url.URL
	apiKey  string
	http    *http.Client
}

type Response struct {
	StatusCode  int
	Header      http.Header
	Body        []byte
	ContentType string
}

func NewClient(baseURL, apiKey string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL: u,
		apiKey:  apiKey,
		http: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (c *Client) Get(ctx context.Context, path string, query url.Values) (Response, error) {
	return c.do(ctx, http.MethodGet, path, query, nil)
}

func (c *Client) PostJSON(ctx context.Context, path string, body []byte) (Response, error) {
	return c.do(ctx, http.MethodPost, path, nil, body)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body []byte) (Response, error) {
	u := *c.baseURL
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(path, "/")
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), r)
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/json"
	}

	if resp.StatusCode >= 500 {
		return Response{
			StatusCode:  resp.StatusCode,
			Header:      resp.Header.Clone(),
			Body:        b,
			ContentType: ct,
		}, fmt.Errorf("teldog upstream status=%d", resp.StatusCode)
	}

	return Response{
		StatusCode:  resp.StatusCode,
		Header:      resp.Header.Clone(),
		Body:        b,
		ContentType: ct,
	}, nil
}

