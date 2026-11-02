package dctl

import (
	"context"
	"net/http"
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
	if c.Method != http.MethodPost || c.Path != "/guilds/g1/channels" {
		t.Errorf("call = %s %s", c.Method, c.Path)
	}
	if asType[map[string]any](t, c.Body)["name"] != "logs" {
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
	if c.Method != http.MethodPatch || c.Path != "/channels/c1" {
		t.Errorf("call = %s %s", c.Method, c.Path)
	}
}

func TestChannelsEnsureUnderReuses(t *testing.T) {
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
	if c := s.Last(); c.Method != http.MethodGet {
		t.Errorf("reuse should not create, last call = %s %s", c.Method, c.Path)
	}
}

func TestChannelsEnsureUnderCreates(t *testing.T) {
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
	if c.Method != http.MethodPost || c.Path != "/guilds/g1/channels" {
		t.Fatalf("create call = %s %s", c.Method, c.Path)
	}
	if asType[map[string]any](t, c.Body)["parent_id"] != "p1" {
		t.Errorf("body = %v", c.Body)
	}
	if n := len(s.Calls()); n != 3 {
		t.Errorf("made %d calls, want 3 (parent, list, create)", n)
	}
}

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
	if c := s.Last(); c.Method != http.MethodDelete || c.Path != "/channels/c1" {
		t.Errorf("call = %s %s", c.Method, c.Path)
	}
}
