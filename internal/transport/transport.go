// Package transport is dctl's HTTP boundary to the Discord REST API (v10):
// auth, request building, error decoding. It is the single mockable seam —
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
	h := &HTTP{token: token, base: DefaultBase, client: &http.Client{Timeout: 15 * time.Second}}
	for _, o := range opts {
		o(h)
	}
	return h
}

// Enabled reports whether a token is configured.
func (h *HTTP) Enabled() bool { return h != nil && h.token != "" }

// scrubURLError strips the request URL from a transport error. net/http returns a
// *url.Error whose Error() embeds the full URL, and several Discord paths carry
// secret tokens (webhook/interaction tokens) as path segments — echoing that URL
// into a caller's log would leak them. We keep only the method and the wrapped
// cause (dial/timeout/context error), which is token-free and preserves errors.Is
// against context.DeadlineExceeded / context.Canceled.
func scrubURLError(method string, err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("dctl: %s request failed: %w", method, ue.Err)
	}
	return err
}

// rateLimitRetries is how many times a request waits out a 429 before giving
// up. Discord's buckets are small — five writes per four seconds on some routes
// — so a caller doing a handful of calls in a row will hit one in the ordinary
// course of doing its job, and the answer says exactly how long to wait. Without
// this, the caller sees a failure it can only paper over by retrying the whole
// operation, which walks into the same wall.
const rateLimitRetries = 4

// maxRateLimitWait caps how long one request will sit waiting. Past it, the
// answer is not a bucket refilling in a moment but a daily quota or something
// the operator has to look at, and blocking on it would hide that.
const maxRateLimitWait = 30 * time.Second

func (h *HTTP) Do(ctx context.Context, method, path string, body, out any) error {
	if !h.Enabled() {
		return ErrDisabled
	}
	var buf []byte
	if body != nil {
		var err error
		if buf, err = json.Marshal(body); err != nil {
			return err
		}
	}
	for attempt := 0; ; attempt++ {
		respBody, status, err := h.attempt(ctx, method, path, buf, body != nil)
		if err != nil {
			return err
		}
		if status == http.StatusTooManyRequests && attempt < rateLimitRetries {
			wait, ok := retryAfter(respBody)
			if ok && wait <= maxRateLimitWait {
				select {
				case <-time.After(wait):
					continue
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
		if status < 200 || status >= 300 {
			return &APIError{Status: status, Body: strings.TrimSpace(string(respBody))}
		}
		if out == nil || len(respBody) == 0 {
			return nil
		}
		return json.Unmarshal(respBody, out)
	}
}

// retryAfter reads how long Discord asked us to wait. A 429 body carries it in
// seconds, fractional. An unreadable body reports false rather than guessing: a
// wait invented here would either hammer the route or stall the caller.
func retryAfter(body []byte) (time.Duration, bool) {
	var v struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if err := json.Unmarshal(body, &v); err != nil || v.RetryAfter <= 0 {
		return 0, false
	}
	// Discord's clock and ours disagree by a little, and coming back a hair early
	// spends the retry for nothing.
	return time.Duration(v.RetryAfter*float64(time.Second)) + 250*time.Millisecond, true
}

// attempt performs one request and hands back the raw response. It separates
// what is worth retrying (a status) from what is not (a transport failure).
func (h *HTTP) attempt(ctx context.Context, method, path string, buf []byte, hasBody bool) ([]byte, int, error) {
	var rdr io.Reader
	if hasBody {
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, h.base+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bot "+h.token)
	req.Header.Set("User-Agent", "dctl (https://github.com/Herrscherd/dctl, 1.0)")
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, 0, scrubURLError(method, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, 0, fmt.Errorf("reading response: %w", err)
	}
	return respBody, resp.StatusCode, nil
}
