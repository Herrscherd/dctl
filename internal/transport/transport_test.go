package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		writeBody(t, w, `{"message":"Missing Permissions"}`)
	}))
	defer srv.Close()
	rt := NewHTTP("tok", WithBase(srv.URL))
	err := rt.Do(context.Background(), http.MethodGet, "/x", nil, nil)
	if err == nil {
		t.Fatal("want error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T %v, want *APIError", err, err)
	}
	if apiErr.Status != http.StatusForbidden {
		t.Errorf("status = %d, want %d", apiErr.Status, http.StatusForbidden)
	}
	if !strings.Contains(apiErr.Body, "Missing Permissions") {
		t.Errorf("body = %q, want the Discord message", apiErr.Body)
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

func TestHTTPDoAcceptsASuccessWhoseBodyFailsToClose(t *testing.T) {
	closeErr := errors.New("body close failed")
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       closeFailingBody{Reader: strings.NewReader(`{"id":"42"}`), err: closeErr},
		}, nil
	})}
	var logs strings.Builder
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	rt := NewHTTP("tok", WithHTTPClient(client), WithBase("http://discord.invalid"), WithLogger(logger))
	var out struct {
		ID string `json:"id"`
	}
	if err := rt.Do(context.Background(), http.MethodPost, "/x", map[string]string{"content": "hi"}, &out); err != nil {
		t.Fatalf("err = %v, want success: the accepted write must not look failed", err)
	}
	if out.ID != "42" {
		t.Errorf("id = %q, want the decoded reply", out.ID)
	}
	if !strings.Contains(logs.String(), closeErr.Error()) {
		t.Errorf("logs = %q, want the close failure logged", logs.String())
	}
}

func TestHTTPDoReportsANotFoundWhenBodyFailsToClose(t *testing.T) {
	closeErr := errors.New("body close failed")
	var logs strings.Builder
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       closeFailingBody{Reader: strings.NewReader(`{"message":"Unknown Message"}`), err: closeErr},
		}, nil
	})}
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	rt := NewHTTP("tok", WithHTTPClient(client), WithBase("http://discord.invalid"), WithLogger(logger))
	err := rt.Do(context.Background(), http.MethodGet, "/x", nil, nil)
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok || apiErr.Status != http.StatusNotFound {
		t.Fatalf("err = %v, want *APIError with status 404", err)
	}
	if !strings.Contains(logs.String(), closeErr.Error()) {
		t.Errorf("logs = %q, want the close failure logged", logs.String())
	}
}

func TestHTTPDoRetriesARateLimitWhenBodyFailsToClose(t *testing.T) {
	closeErr := errors.New("body close failed")
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       closeFailingBody{Reader: strings.NewReader(`{"retry_after":0.01}`), err: closeErr},
			}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	rt := NewHTTP("tok", WithHTTPClient(client), WithBase("http://discord.invalid"))
	if err := rt.Do(context.Background(), http.MethodGet, "/x", nil, nil); err != nil {
		t.Fatalf("err = %v, want the rate limit retried to success", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want the 429 replayed once", calls)
	}
}

func TestHTTPDoReportsANotFoundWhenBodyFailsToRead(t *testing.T) {
	readErr := errors.New("connection reset")
	const partial = `{"message":"Unknown Message"`
	var logs strings.Builder
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(io.MultiReader(strings.NewReader(partial), iotest.ErrReader(readErr))),
		}, nil
	})}
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	rt := NewHTTP("tok", WithHTTPClient(client), WithBase("http://discord.invalid"), WithLogger(logger))
	err := rt.Do(context.Background(), http.MethodGet, "/x", nil, nil)
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok || apiErr.Status != http.StatusNotFound {
		t.Fatalf("err = %v, want *APIError with status 404", err)
	}
	if apiErr.Body != partial {
		t.Errorf("body = %q, want the partial body kept", apiErr.Body)
	}
	if !strings.Contains(logs.String(), readErr.Error()) {
		t.Errorf("logs = %q, want the read failure logged", logs.String())
	}
}

func TestHTTPDoRetriesARateLimitWhenBodyFailsToRead(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body: io.NopCloser(io.MultiReader(
					strings.NewReader(`{"retry_after":0.01}`),
					iotest.ErrReader(errors.New("connection reset")),
				)),
			}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	rt := NewHTTP("tok", WithHTTPClient(client), WithBase("http://discord.invalid"))
	if err := rt.Do(context.Background(), http.MethodGet, "/x", nil, nil); err != nil {
		t.Fatalf("err = %v, want the rate limit retried to success", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want the 429 replayed once", calls)
	}
}

func writeBody(tb testing.TB, w io.Writer, body string) {
	tb.Helper()
	if _, err := io.WriteString(w, body); err != nil {
		tb.Errorf("writing response: %v", err)
	}
}

func TestEncodeBodyEncodesAMultipartByValueOrPointer(t *testing.T) {
	multi := Multipart{
		Payload: map[string]any{"content": "hi"},
		Files:   []Part{{Name: "files[0]", Filename: `a"b.png`, Content: strings.NewReader("PNG")}},
	}
	for name, body := range map[string]any{"value": multi, "pointer": &multi} {
		t.Run(name, func(t *testing.T) {
			multi.Files[0].Content = strings.NewReader("PNG")
			got, err := encodeBody(body)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(got.contentType, "multipart/form-data; boundary=") {
				t.Fatalf("content type = %q, want multipart/form-data", got.contentType)
			}
			if want := `Content-Disposition: form-data; name="files[0]"; filename="a\"b.png"`; !strings.Contains(string(got.data), want) {
				t.Errorf("body lacks %q:\n%s", want, got.data)
			}
		})
	}
}
