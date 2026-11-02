// Package transport is dctl's HTTP boundary to the Discord REST API (v10):
// auth, request building, error decoding. It is the single mockable seam:
// resource clients depend on Doer, never on net/http directly.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBase is the Discord REST API root.
const DefaultBase = "https://discord.com/api/v10"

const (
	defaultTimeout   = 15 * time.Second
	maxResponseBytes = 1 << 20
)

// ErrDisabled is returned by Do when no bot token is configured.
var ErrDisabled = errors.New("dctl: no bot token (DISCORD_BOT_TOKEN)")

// APIError is a non-2xx response from the Discord REST API.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string { return fmt.Sprintf("discord %d: %s", e.Status, e.Body) }

// Doer performs one Discord REST call: it marshals body (if non-nil), executes
// method+path against the API, and decodes the JSON response into out (if non-nil).
type Doer interface {
	Do(ctx context.Context, method, path string, body, out any) error
	Enabled() bool
}

// HTTP is the real Doer.
type HTTP struct {
	token  string
	base   string
	client *http.Client
}

// Option configures an HTTP transport.
type Option func(*HTTP)

// WithBase overrides the API root (used by tests).
func WithBase(base string) Option { return func(h *HTTP) { h.base = base } }

// WithHTTPClient overrides the default 15s-timeout client.
func WithHTTPClient(c *http.Client) Option { return func(h *HTTP) { h.client = c } }

// NewHTTP builds the real transport. An empty token makes every Do return ErrDisabled.
func NewHTTP(token string, opts ...Option) *HTTP {
	httpTransport := &HTTP{token: token, base: DefaultBase, client: &http.Client{Timeout: defaultTimeout}}
	for _, o := range opts {
		o(httpTransport)
	}
	return httpTransport
}

// Enabled reports whether a token is configured.
func (h *HTTP) Enabled() bool { return h != nil && h.token != "" }

func scrubURLError(method string, err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("dctl: %s request failed: %w", method, urlErr.Err)
	}
	return err
}

const rateLimitRetries = 4

const maxRateLimitWait = 30 * time.Second

// Do sends one request to the REST API and decodes the JSON reply into out. It
// returns [ErrDisabled] when no token is set, a [*APIError] for a non-2xx status,
// and the context error when ctx ends while a rate limit is waited out.
func (h *HTTP) Do(ctx context.Context, method, path string, body, out any) error {
	if !h.Enabled() {
		return ErrDisabled
	}
	buf, err := marshalBody(body)
	if err != nil {
		return err
	}
	for attempt := 0; ; attempt++ {
		resp, err := h.attempt(ctx, method, path, buf, body != nil)
		if err != nil {
			return err
		}
		wait, retry := retryDelay(resp.status, attempt, resp.body)
		if !retry {
			return decodeResponse(resp.status, resp.body, out)
		}
		if err := waitFor(ctx, wait); err != nil {
			return err
		}
	}
}

func retryDelay(status, attempt int, respBody []byte) (time.Duration, bool) {
	if status != http.StatusTooManyRequests || attempt >= rateLimitRetries {
		return 0, false
	}
	wait, ok := retryAfter(respBody)
	return wait, ok && wait <= maxRateLimitWait
}

func marshalBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	return json.Marshal(body)
}

func waitFor(ctx context.Context, wait time.Duration) error {
	select {
	case <-time.After(wait):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func decodeResponse(status int, respBody []byte, out any) error {
	if status < 200 || status >= 300 {
		return &APIError{Status: status, Body: strings.TrimSpace(string(respBody))}
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	return json.Unmarshal(respBody, out)
}

func retryAfter(body []byte) (time.Duration, bool) {
	var rateLimit struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if err := json.Unmarshal(body, &rateLimit); err != nil || rateLimit.RetryAfter <= 0 {
		return 0, false
	}
	return time.Duration(rateLimit.RetryAfter*float64(time.Second)) + 250*time.Millisecond, true
}

type response struct {
	body   []byte
	status int
}

func (h *HTTP) attempt(ctx context.Context, method, path string, buf []byte, hasBody bool) (response, error) {
	var rdr io.Reader
	if hasBody {
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, h.base+path, rdr)
	if err != nil {
		return response{}, err
	}
	req.Header.Set("Authorization", "Bot "+h.token)
	req.Header.Set("User-Agent", "dctl (https://github.com/Herrscherd/dctl, 1.0)")
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return response{}, scrubURLError(method, err)
	}
	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	closeErr := resp.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return response{}, fmt.Errorf("reading response: %w", err)
	}
	return response{body: respBody, status: resp.StatusCode}, nil
}
