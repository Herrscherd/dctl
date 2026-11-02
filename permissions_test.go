package dctl

import (
	"context"
	"net/http"
	"testing"

	"github.com/Herrscherd/dctl/internal/transport"
)

func TestPermissionsSetOverwrite(t *testing.T) {
	s := transport.NewStub()
	p := &Permissions{rt: s}
	err := p.Set(context.Background(), "c1", "r1", 0, "1024", "0")
	if err != nil {
		t.Fatal(err)
	}
	c := s.Last()
	if c.Method != http.MethodPut || c.Path != "/channels/c1/permissions/r1" {
		t.Errorf("call = %s %s", c.Method, c.Path)
	}
	body := asType[map[string]any](t, c.Body)
	if body["allow"] != "1024" || body["type"] != 0 || body["deny"] != "0" {
		t.Errorf("body = %v", body)
	}
}

func TestPermissionsRemoveOverwrite(t *testing.T) {
	s := transport.NewStub()
	p := &Permissions{rt: s}
	if err := p.Remove(context.Background(), "c1", "r1"); err != nil {
		t.Fatal(err)
	}
	c := s.Last()
	if c.Method != http.MethodDelete || c.Path != "/channels/c1/permissions/r1" {
		t.Errorf("call = %s %s", c.Method, c.Path)
	}
}
