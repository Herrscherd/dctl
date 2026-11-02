package transport

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestStubCapturesAndReplies(t *testing.T) {
	s := NewStub()
	s.Reply(`{"id":"7"}`)
	var out struct {
		ID string `json:"id"`
	}
	err := s.Do(context.Background(), http.MethodPost, "/channels/1/messages", map[string]any{"content": "hi"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "7" {
		t.Errorf("id = %q", out.ID)
	}
	call := s.Last()
	if call.Method != http.MethodPost || call.Path != "/channels/1/messages" {
		t.Errorf("captured %s %s", call.Method, call.Path)
	}
	if mapBody(t, call.Body)["content"] != "hi" {
		t.Errorf("body = %v", call.Body)
	}
}

func TestStubReturnsConfiguredError(t *testing.T) {
	s := NewStub()
	s.Fail(context.DeadlineExceeded)
	if err := s.Do(context.Background(), http.MethodGet, "/x", nil, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v", err)
	}
}

func mapBody(tb testing.TB, body any) map[string]any {
	tb.Helper()
	m, ok := body.(map[string]any)
	if !ok {
		tb.Fatalf("body is %T, want map[string]any", body)
	}
	return m
}
