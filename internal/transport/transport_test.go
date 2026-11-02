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
		writeBody(t, w, `{"id":"42"}`)
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
	if gotMethod != http.MethodGet || gotPath != "/x" {
		t.Errorf("method/path = %s %s", gotMethod, gotPath)
	}
	if out.ID != "42" {
		t.Errorf("id = %q", out.ID)
	}
}

func TestHTTPDoDisabledWithoutToken(t *testing.T) {
	rt := NewHTTP("")
	if err := rt.Do(context.Background(), http.MethodGet, "/x", nil, nil); !errors.Is(err, ErrDisabled) {
		t.Errorf("err = %v, want ErrDisabled", err)
	}
}

func TestHTTPDoSurfacesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		writeBody(t, w, `{"message":"Missing Permissions"}`)
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

func TestHTTPDoTransportErrorDoesNotLeakToken(t *testing.T) {
	const secret = "SUPERSECRETWEBHOOKTOKEN"
	rt := NewHTTP("bottok", WithBase("http://127.0.0.1:1"))
	err := rt.Do(context.Background(), http.MethodPost, "/webhooks/123/"+secret, nil, nil)
	if err == nil {
		t.Fatal("want transport error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaks token: %q", err.Error())
	}
	if !strings.Contains(err.Error(), http.MethodPost) {
		t.Fatalf("error dropped method context: %q", err.Error())
	}
}

func TestHTTPDoMarshalsBody(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		writeBody(t, w, `{}`)
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

func TestHTTPDoWaitsOutARateLimit(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			writeBody(t, w, `{"message":"You are being rate limited.","retry_after":0.01}`)
			return
		}
		writeBody(t, w, `{"id":"42"}`)
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

func TestARetriedWriteCarriesItsBodyAgain(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request: %v", err)
		}
		bodies = append(bodies, string(b))
		if len(bodies) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			writeBody(t, w, `{"retry_after":0.01}`)
			return
		}
		writeBody(t, w, `{}`)
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

func TestALongRateLimitIsReportedRatherThanWaitedOut(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusTooManyRequests)
		writeBody(t, w, `{"retry_after":3600}`)
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

func TestARateLimitThatNeverClearsGivesUp(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusTooManyRequests)
		writeBody(t, w, `{"retry_after":0.001}`)
	}))
	defer srv.Close()

	if err := NewHTTP("tok", WithBase(srv.URL)).Do(context.Background(), http.MethodGet, "/x", nil, nil); err == nil {
		t.Fatal("a limit that never clears must be reported")
	}
	if calls != rateLimitRetries+1 {
		t.Fatalf("calls = %d, want %d", calls, rateLimitRetries+1)
	}
}

func TestAWaitingRequestStopsWithItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel()
		w.WriteHeader(http.StatusTooManyRequests)
		writeBody(t, w, `{"retry_after":20}`)
	}))
	defer srv.Close()

	if err := NewHTTP("tok", WithBase(srv.URL)).Do(ctx, http.MethodGet, "/x", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the cancellation", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type closeFailingBody struct {
	io.Reader

	err error
}

func (b closeFailingBody) Close() error { return b.err }

func TestHTTPDoJoinsBodyCloseError(t *testing.T) {
	closeErr := errors.New("body close failed")
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       closeFailingBody{Reader: strings.NewReader(`{}`), err: closeErr},
		}, nil
	})}
	rt := NewHTTP("tok", WithHTTPClient(client), WithBase("http://discord.invalid"))
	err := rt.Do(context.Background(), http.MethodGet, "/x", nil, nil)
	if !errors.Is(err, closeErr) {
		t.Fatalf("err = %v, want the body close error", err)
	}
}

func writeBody(tb testing.TB, w io.Writer, body string) {
	tb.Helper()
	if _, err := io.WriteString(w, body); err != nil {
		tb.Errorf("writing response: %v", err)
	}
}
