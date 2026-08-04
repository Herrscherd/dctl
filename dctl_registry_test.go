package dctl

import (
	"context"
	"testing"

	"github.com/Herrscherd/dctl/internal/transport"
)

// The registry must be stable across c.Interactions() / .Registry() calls, so
// bindings registered at setup are visible at dispatch time.
func TestClientRegistryIsStable(t *testing.T) {
	c := newWith(transport.NewStub(), "chan")
	c.Interactions().Registry().Add(NewCommand("ping", "p"),
		func(ctx context.Context, ix Interaction) (Response, error) {
			return Response{Content: "pong"}, nil
		})

	r, err := c.Interactions().Registry().Dispatch(context.Background(),
		Interaction{Data: InteractionData{Name: "ping"}})
	if err != nil || r.Content != "pong" {
		t.Fatalf("re-derived registry lost its binding: r=%v err=%v", r, err)
	}
}

// Commands default to the guild scope: instant propagation, one server.
func TestCommandsBaseDefaultsToGuildScope(t *testing.T) {
	s := transport.NewStub().Reply(`{"id":"app1"}`).Reply(`[{"id":"g1","name":"srv"}]`)
	c := newWith(s, "chan")

	base, err := c.Interactions().commandsBase(context.Background())
	if err != nil || base != "/applications/app1/guilds/g1/commands" {
		t.Fatalf("base = %q, %v", base, err)
	}
}

// A bot installable anywhere cannot pin its commands to one server. Global
// scope must also skip the guild lookup entirely — that lookup fails outright
// once the bot is in more than one server.
func TestGlobalCommandsScopeSkipsGuildLookup(t *testing.T) {
	s := transport.NewStub().Reply(`{"id":"app1"}`)
	c := newWith(s, "chan")
	c.def.globalCommands = true

	base, err := c.Interactions().commandsBase(context.Background())
	if err != nil || base != "/applications/app1/commands" {
		t.Fatalf("base = %q, %v", base, err)
	}
	for _, call := range s.Calls() {
		if call.Path == "/users/@me/guilds" {
			t.Fatal("global commands must not resolve a guild")
		}
	}
}

func TestWithGlobalCommandsConfiguresDefault(t *testing.T) {
	if c := New("token", "chan", WithGlobalCommands()); !c.def.globalCommands {
		t.Error("WithGlobalCommands did not reach the client")
	}
	if c := New("token", "chan"); c.def.globalCommands {
		t.Error("commands must stay guild-scoped without the option")
	}
}

// AppID is fetched once and cached, even across separate sub-client ops.
func TestAppIDCached(t *testing.T) {
	s := transport.NewStub().Reply(`{"id":"app1"}`).Reply(`{"id":"app1"}`)
	c := newWith(s, "chan")

	for i := 0; i < 3; i++ {
		if id, err := c.Interactions().AppID(context.Background()); err != nil || id != "app1" {
			t.Fatalf("AppID = %q, %v", id, err)
		}
	}
	hits := 0
	for _, call := range s.Calls() {
		if call.Path == "/users/@me" {
			hits++
		}
	}
	if hits != 1 {
		t.Fatalf("/users/@me fetched %d times, want 1 (cached)", hits)
	}
}
