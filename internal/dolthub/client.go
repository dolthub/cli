package dolthub

import (
	"bytes"
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

const (
	maxSuccessResponse = 16 << 20
	maxErrorResponse   = 1 << 20
)

// Client is a DoltHub REST API v2 client.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
}

// Meta is optional response metadata returned by the v2 API.
type Meta struct {
	NextPageToken string `json:"next_page_token"`
}

// RawResponse is a successful generic API response.
type RawResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// Raw performs a generic request beneath the configured API v2 prefix.
func (c *Client) Raw(ctx context.Context, method, endpoint string, body []byte) (RawResponse, error) {
	reference, err := url.Parse(endpoint)
	if err != nil || reference.IsAbs() || reference.Host != "" || reference.Fragment != "" {
		return RawResponse{}, errors.New("API endpoint must be relative to /api/v2/")
	}
	clean := strings.TrimPrefix(reference.Path, "/api/v2/")
	clean = strings.TrimPrefix(clean, "api/v2/")
	if strings.HasPrefix(reference.Path, "/") && !strings.HasPrefix(reference.Path, "/api/v2/") {
		return RawResponse{}, errors.New("API endpoint must be relative to /api/v2/")
	}
	for _, segment := range strings.Split(clean, "/") {
		if segment == ".." || segment == "." {
			return RawResponse{}, errors.New("API endpoint must not escape /api/v2/")
		}
	}
	reference.Path = clean
	reference.RawPath = ""
	u := c.baseURL.ResolveReference(reference)
	if !strings.HasPrefix(u.Path, c.baseURL.Path) {
		return RawResponse{}, errors.New("API endpoint must not escape /api/v2/")
	}
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return RawResponse{}, err
	}
	req.Header.Set("Accept", "application/json")
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return RawResponse{}, ctx.Err()
		}
		return RawResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return RawResponse{}, decodeAPIError(resp, req)
	}
	payload, err := readLimited(resp.Body, maxSuccessResponse)
	if err != nil {
		return RawResponse{}, err
	}
	return RawResponse{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: payload}, nil
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
	if baseURL.Host == "" || baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, errors.New("base URL must be an HTTP origin and path")
	}
	u := *baseURL
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	return &Client{httpClient: httpClient, baseURL: &u}, nil
}

// path joins API path segments while escaping each segment independently.
func path(segments ...string) string {
	escaped := make([]string, len(segments))
	for i, segment := range segments {
		escaped[i] = url.PathEscape(segment)
	}
	return strings.Join(escaped, "/")
}

// resolveSameOrigin resolves a URL without allowing an authenticated client to
// send credentials to another origin.
func (c *Client) resolveSameOrigin(raw string) (*url.URL, error) {
	reference, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse DoltHub URL: %w", err)
	}
	resolved := c.baseURL.ResolveReference(reference)
	if resolved.Scheme != c.baseURL.Scheme || !strings.EqualFold(resolved.Host, c.baseURL.Host) || resolved.User != nil {
		return nil, errors.New("DoltHub URL must use the configured origin")
	}
	if resolved.Fragment != "" {
		return nil, errors.New("DoltHub URL must not contain a fragment")
	}
	return resolved, nil
}

func (c *Client) get(ctx context.Context, requestPath string, out any) error {
	_, err := c.request(ctx, http.MethodGet, requestPath, nil, nil, out)
	return err
}

func (c *Client) request(ctx context.Context, method, requestPath string, query url.Values, body, out any) (Meta, error) {
	u, err := c.resolveSameOrigin(requestPath)
	if err != nil {
		return Meta{}, err
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return Meta{}, fmt.Errorf("encode DoltHub request: %w", err)
		}
		requestBody = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), requestBody)
	if err != nil {
		return Meta{}, fmt.Errorf("create DoltHub request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Meta{}, ctx.Err()
		}
		return Meta{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Meta{}, decodeAPIError(resp, req)
	}

	payload, err := readLimited(resp.Body, maxSuccessResponse)
	if err != nil {
		return Meta{}, fmt.Errorf("read DoltHub response: %w", err)
	}
	var raw struct {
		Data json.RawMessage `json:"data"`
		Meta Meta            `json:"meta"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return Meta{}, fmt.Errorf("decode DoltHub response: %w", err)
	}
	if raw.Data == nil {
		return Meta{}, errors.New("decode DoltHub response: response has no data field")
	}
	if out != nil {
		if err := json.Unmarshal(raw.Data, out); err != nil {
			return Meta{}, fmt.Errorf("decode DoltHub response data: %w", err)
		}
	}
	return raw.Meta, nil
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > limit {
		return nil, fmt.Errorf("body exceeds %d byte limit", limit)
	}
	return payload, nil
}

type problem struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail"`
	Code      string `json:"code"`
	RequestID string `json:"request_id"`
}

func decodeAPIError(resp *http.Response, req *http.Request) error {
	p := problem{}
	payload, _ := readLimited(resp.Body, maxErrorResponse)
	_ = json.Unmarshal(payload, &p)
	requestID := resp.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = p.RequestID
	}
	return &APIError{Status: resp.StatusCode, Method: req.Method, Path: req.URL.EscapedPath(), RequestID: requestID, Type: p.Type, Title: safeServerText(p.Title), Detail: safeServerText(p.Detail), Code: p.Code}
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
