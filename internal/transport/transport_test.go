package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPDoSetsAuthAndDecodes(t *testing.T) {
	var gotAuth, gotUA, gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Write([]byte(`{"id":"42"}`))
	}))
	defer srv.Close()

	rt := NewHTTP("tok", WithBase(srv.URL))
	var out struct {
		ID string `json:"id"`
	}
	if err := rt.Do(context.Background(), http.MethodGet, "/x", nil, &out); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bot tok" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotUA == "" {
		t.Error("missing user-agent")
	}
	if gotMethod != "GET" || gotPath != "/x" {
		t.Errorf("method/path = %s %s", gotMethod, gotPath)
	}
	if out.ID != "42" {
		t.Errorf("id = %q", out.ID)
	}
}

func TestHTTPDoDisabledWithoutToken(t *testing.T) {
	rt := NewHTTP("")
	if err := rt.Do(context.Background(), http.MethodGet, "/x", nil, nil); err != ErrDisabled {
		t.Errorf("err = %v, want ErrDisabled", err)
	}
}

func TestHTTPDoSurfacesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"Missing Permissions"}`))
	}))
	defer srv.Close()
	rt := NewHTTP("tok", WithBase(srv.URL))
	err := rt.Do(context.Background(), http.MethodGet, "/x", nil, nil)
	if err == nil {
		t.Fatal("want error")
	}
	if want := "discord 403"; !strings.Contains(err.Error(), want) {
		t.Errorf("err = %q, want containing %q", err.Error(), want)
	}
}

// A transport-layer failure (here: connection refused) must not echo the request
// URL, whose path carries secret tokens (webhook/interaction tokens) into any log
// the caller writes. net/http returns a *url.Error whose Error() embeds the full
// URL; Do must scrub it.
func TestHTTPDoTransportErrorDoesNotLeakToken(t *testing.T) {
	const secret = "SUPERSECRETWEBHOOKTOKEN"
	// 127.0.0.1:1 refuses connections, forcing client.Do to fail.
	rt := NewHTTP("bottok", WithBase("http://127.0.0.1:1"))
	err := rt.Do(context.Background(), http.MethodPost, "/webhooks/123/"+secret, nil, nil)
	if err == nil {
		t.Fatal("want transport error")
	}
	// The secret token (a path segment) must never survive into the error string.
	// The destination host may remain (it is the public API base, not a secret).
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaks token: %q", err.Error())
	}
	// Cause is preserved for errors.Is against context/network errors.
	if !strings.Contains(err.Error(), "POST") {
		t.Fatalf("error dropped method context: %q", err.Error())
	}
}

func TestHTTPDoMarshalsBody(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	rt := NewHTTP("tok", WithBase(srv.URL))
	if err := rt.Do(context.Background(), http.MethodPost, "/x", map[string]any{"content": "hi"}, nil); err != nil {
		t.Fatal(err)
	}
	if got["content"] != "hi" {
		t.Errorf("body content = %v", got["content"])
	}
}

// Discord's buckets are small — five writes per four seconds on some routes — so
// a caller doing a handful of calls in a row hits one while doing its job. The
// answer says how long to wait, and waiting it out is the whole fix.
func TestHTTPDoWaitsOutARateLimit(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"message":"You are being rate limited.","retry_after":0.01}`))
			return
		}
		w.Write([]byte(`{"id":"42"}`))
	}))
	defer srv.Close()

	var out struct {
		ID string `json:"id"`
	}
	if err := NewHTTP("tok", WithBase(srv.URL)).Do(context.Background(), http.MethodGet, "/x", nil, &out); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || out.ID != "42" {
		t.Fatalf("calls = %d, id = %q, want the request replayed after the wait", calls, out.ID)
	}
}

// A body is consumable, so replaying a write means rebuilding it. Sending the
// second attempt with an empty body would be worse than not retrying at all.
func TestARetriedWriteCarriesItsBodyAgain(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if len(bodies) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"retry_after":0.01}`))
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	if err := NewHTTP("tok", WithBase(srv.URL)).Do(context.Background(), http.MethodPost, "/x",
		map[string]string{"name": "mode"}, nil); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 || bodies[0] != bodies[1] {
		t.Fatalf("bodies = %q, want the same body sent twice", bodies)
	}
}

// Past a point the answer is not a bucket refilling in a moment but a quota, and
// blocking on it would hide that from the caller.
func TestALongRateLimitIsReportedRatherThanWaitedOut(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"retry_after":3600}`))
	}))
	defer srv.Close()

	err := NewHTTP("tok", WithBase(srv.URL)).Do(context.Background(), http.MethodGet, "/x", nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusTooManyRequests {
		t.Fatalf("err = %v, want the 429 reported", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want no waiting on an hour-long limit", calls)
	}
}

// A route that stays limited must not retry forever: the caller is waiting on a
// call that is never going to land.
func TestARateLimitThatNeverClearsGivesUp(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"retry_after":0.001}`))
	}))
	defer srv.Close()

	if err := NewHTTP("tok", WithBase(srv.URL)).Do(context.Background(), http.MethodGet, "/x", nil, nil); err == nil {
		t.Fatal("a limit that never clears must be reported")
	}
	if calls != rateLimitRetries+1 {
		t.Fatalf("calls = %d, want %d", calls, rateLimitRetries+1)
	}
}

// A cancelled context must not be held by a request sitting out a wait.
func TestAWaitingRequestStopsWithItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel()
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"retry_after":20}`))
	}))
	defer srv.Close()

	if err := NewHTTP("tok", WithBase(srv.URL)).Do(ctx, http.MethodGet, "/x", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the cancellation", err)
	}
}
