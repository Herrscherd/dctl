package dctl

import (
	"context"
	"testing"

	"github.com/Herrscherd/dctl/internal/transport"
)

func chans(s *transport.Stub) *Channels {
	return &Channels{rt: s, def: &defaults{guilds: &Guilds{rt: s}}}
}

func TestChannelsCreate(t *testing.T) {
	s := transport.NewStub().
		Reply(`{"id":"c1","name":"logs","type":0}`)
	ch, err := chans(s).Create(context.Background(), "g1", "logs")
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID != "c1" {
		t.Fatalf("channel = %+v", ch)
	}
	c := s.Last()
	if c.Method != "POST" || c.Path != "/guilds/g1/channels" {
		t.Errorf("call = %s %s", c.Method, c.Path)
	}
	if c.Body.(map[string]any)["name"] != "logs" {
		t.Errorf("body = %v", c.Body)
	}
}

func TestChannelsRename(t *testing.T) {
	s := transport.NewStub().Reply(`{"id":"c1","name":"new"}`)
	ch, err := chans(s).Rename(context.Background(), "c1", "new")
	if err != nil {
		t.Fatal(err)
	}
	if ch.Name != "new" {
		t.Fatalf("name = %q", ch.Name)
	}
	c := s.Last()
	if c.Method != "PATCH" || c.Path != "/channels/c1" {
		t.Errorf("call = %s %s", c.Method, c.Path)
	}
}

func TestChannelsEnsureUnderReuses(t *testing.T) {
	// The parent category names its own guild, then the channel listing holds a
	// match under p1.
	s := transport.NewStub().
		Reply(`{"id":"p1","guild_id":"g1"}`).
		Reply(`[{"id":"c9","name":"Notes","type":0,"parent_id":"p1"}]`)
	ch, err := chans(s).EnsureUnder(context.Background(), "p1", "notes")
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID != "c9" {
		t.Fatalf("expected reuse of c9, got %+v", ch)
	}
	if c := s.Last(); c.Method != "GET" {
		t.Errorf("reuse should not create, last call = %s %s", c.Method, c.Path)
	}
}

func TestChannelsEnsureUnderCreates(t *testing.T) {
	// No match under p1 (the existing channel sits under a different parent), so
	// it creates one nested under p1 — in the guild the parent already named, with
	// no second lookup of it.
	s := transport.NewStub().
		Reply(`{"id":"p1","guild_id":"g1"}`).
		Reply(`[{"id":"c9","name":"notes","type":0,"parent_id":"other"}]`).
		Reply(`{"id":"c10","name":"notes","type":0,"parent_id":"p1"}`)
	ch, err := chans(s).EnsureUnder(context.Background(), "p1", "notes")
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID != "c10" {
		t.Fatalf("expected new c10, got %+v", ch)
	}
	c := s.Last()
	if c.Method != "POST" || c.Path != "/guilds/g1/channels" {
		t.Fatalf("create call = %s %s", c.Method, c.Path)
	}
	if c.Body.(map[string]any)["parent_id"] != "p1" {
		t.Errorf("body = %v", c.Body)
	}
	if n := len(s.Calls()); n != 3 {
		t.Errorf("made %d calls, want 3 (parent, list, create)", n)
	}
}

// A bot in several servers has no sole guild: nesting under a category must
// resolve the server from that category, never from the bot's membership list.
func TestChannelsUnderNeverAsksForTheSoleGuild(t *testing.T) {
	s := transport.NewStub().
		Reply(`{"id":"p1","guild_id":"g2"}`).
		Reply(`{"id":"c1","name":"notes","type":0,"parent_id":"p1"}`)
	if _, err := chans(s).CreateUnder(context.Background(), "p1", "notes"); err != nil {
		t.Fatal(err)
	}
	if c := s.Last(); c.Path != "/guilds/g2/channels" {
		t.Errorf("create call = %s, want the parent's guild", c.Path)
	}
	for _, c := range s.Calls() {
		if c.Path == "/users/@me/guilds" {
			t.Fatal("the parent category already names its guild; nothing should list the bot's servers")
		}
	}
}

func TestChannelsDelete(t *testing.T) {
	s := transport.NewStub()
	if err := chans(s).Delete(context.Background(), "c1"); err != nil {
		t.Fatal(err)
	}
	if c := s.Last(); c.Method != "DELETE" || c.Path != "/channels/c1" {
		t.Errorf("call = %s %s", c.Method, c.Path)
	}
}
