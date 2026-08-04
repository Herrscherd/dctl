package dctl

import (
	"context"
	"testing"

	"github.com/Herrscherd/dctl/internal/transport"
)

func TestDefaultsResolveChannelPrefersExplicit(t *testing.T) {
	d := &defaults{channel: "def"}
	got, err := d.resolveChannel("explicit")
	if err != nil || got != "explicit" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestDefaultsResolveChannelFallsBackToDefault(t *testing.T) {
	d := &defaults{channel: "def"}
	got, err := d.resolveChannel("")
	if err != nil || got != "def" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestDefaultsResolveChannelErrorsWhenNone(t *testing.T) {
	d := &defaults{}
	if _, err := d.resolveChannel(""); err != ErrNoChannel {
		t.Fatalf("err = %v, want ErrNoChannel", err)
	}
}

// A configured guild must win over the sole-guild lookup: a bot in several
// servers has no sole guild, and the lookup would fail the call outright.
func TestResolveGuildPrefersConfiguredGuild(t *testing.T) {
	s := transport.NewStub().Reply(`[{"id":"g1","name":"a"},{"id":"g2","name":"b"}]`)
	d := &defaults{guild: "g2", guilds: &Guilds{rt: s}}
	got, err := d.resolveGuild(context.Background(), "")
	if err != nil || got != "g2" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := (&defaults{guilds: &Guilds{rt: s}}).resolveGuild(context.Background(), ""); err == nil {
		t.Fatal("without a configured guild a multi-server bot should still fail")
	}
}

func TestWithGuildConfiguresDefault(t *testing.T) {
	c := New("token", "chan", WithGuild("g7"))
	if c.def.guild != "g7" {
		t.Fatalf("def.guild = %q, want g7", c.def.guild)
	}
	if plain := New("token", "chan"); plain.def.guild != "" {
		t.Fatalf("def.guild = %q, want empty without WithGuild", plain.def.guild)
	}
}

func TestResolveGuildUsesSoleGuild(t *testing.T) {
	s := transport.NewStub().Reply(`[{"id":"g1","name":"srv"}]`)
	d := &defaults{guilds: &Guilds{rt: s}}
	got, err := d.resolveGuild(context.Background(), "")
	if err != nil || got != "g1" {
		t.Fatalf("got %q, %v", got, err)
	}
}
