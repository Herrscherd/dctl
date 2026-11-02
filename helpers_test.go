package dctl

import "testing"

func asType[T any](tb testing.TB, value any) T {
	tb.Helper()
	typed, ok := value.(T)
	if !ok {
		var zero T
		tb.Fatalf("got %T, want %T", value, zero)
	}
	return typed
}
