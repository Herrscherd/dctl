package dctl

import (
	"bytes"
	"encoding/json"
	"testing"
)

func asType[T any](tb testing.TB, value any) T {
	tb.Helper()
	typed, ok := value.(T)
	if !ok {
		var zero T
		tb.Fatalf("got %T, want %T", value, zero)
	}
	return typed
}

func sameJSON(tb testing.TB, got, want any) bool {
	tb.Helper()
	gotJSON, err := json.Marshal(got)
	if err != nil {
		tb.Fatalf("marshal got: %v", err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		tb.Fatalf("marshal want: %v", err)
	}
	return bytes.Equal(gotJSON, wantJSON)
}
