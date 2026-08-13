package dolthub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

// Client is a DoltHub REST API v2 client.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
}

// NewClient constructs a client using an injected HTTP client and API base URL.
func NewClient(httpClient *http.Client, baseURL *url.URL) (*Client, error) {
	if httpClient == nil {
		return nil, errors.New("HTTP client is required")
	}
	if baseURL == nil {
		return nil, errors.New("base URL is required")
	}
	if baseURL.Scheme != "https" && baseURL.Scheme != "http" {
		return nil, errors.New("base URL must use HTTP or HTTPS")
	}
	u := *baseURL
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	return &Client{httpClient: httpClient, baseURL: &u}, nil
}

type envelope[T any] struct {
	Data T `json:"data"`
}
type problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	u, err := c.baseURL.Parse(path)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp, req)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out); err != nil {
		return fmt.Errorf("decode DoltHub response: %w", err)
	}
	return nil
}

func decodeAPIError(resp *http.Response, req *http.Request) error {
	p := problem{}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&p)
	return &APIError{Status: resp.StatusCode, Method: req.Method, Path: req.URL.EscapedPath(), RequestID: resp.Header.Get("X-Request-ID"), Type: p.Type, Title: safeServerText(p.Title), Detail: safeServerText(p.Detail), Code: p.Code}
}

var bearerPattern = regexp.MustCompile(`(?i)bearer\s+[^\s]+`)

func safeServerText(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	return bearerPattern.ReplaceAllString(value, "Bearer [REDACTED]")
}
