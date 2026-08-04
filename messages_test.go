package dctl

import (
	"context"
	"testing"

	"github.com/Herrscherd/dctl/internal/transport"
)

func msgs(s *transport.Stub, def string) *Messages {
	return &Messages{rt: s, def: &defaults{channel: def}}
}

func TestMessagesSendUsesDefaultChannelAndNoAllowedMentions(t *testing.T) {
	s := transport.NewStub().Reply(`{"id":"m1","content":"hi @everyone"}`)
	m, err := msgs(s, "def").Send(context.Background(), "", "hi @everyone")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "m1" {
		t.Fatalf("msg = %+v", m)
	}
	c := s.Last()
	if c.Path != "/channels/def/messages" {
		t.Errorf("path = %s", c.Path)
	}
	if _, present := c.Body.(map[string]any)["allowed_mentions"]; present {
		t.Error("noMentions removed: allowed_mentions must NOT be injected")
	}
}

func TestMessagesReadReversesToChronological(t *testing.T) {
	s := transport.NewStub().Reply(`[{"id":"3"},{"id":"2"},{"id":"1"}]`)
	got, err := msgs(s, "def").Read(context.Background(), "c", 50, "")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != "1" || got[2].ID != "3" {
		t.Fatalf("order = %v", []string{got[0].ID, got[1].ID, got[2].ID})
	}
}

func TestMessagesEdit(t *testing.T) {
	s := transport.NewStub().Reply(`{"id":"m1","content":"new"}`)
	if _, err := msgs(s, "").Edit(context.Background(), "c", "m1", "new"); err != nil {
		t.Fatal(err)
	}
	c := s.Last()
	if c.Method != "PATCH" || c.Path != "/channels/c/messages/m1" {
		t.Errorf("call = %s %s", c.Method, c.Path)
	}
}

func TestMessagesGetFetchesOneMessageByID(t *testing.T) {
	s := transport.NewStub().Reply(`{"id":"m1","channel_id":"c","content":"look at this"}`)
	got, err := msgs(s, "def").Get(context.Background(), "c", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "m1" || got.Content != "look at this" {
		t.Fatalf("msg = %+v", got)
	}
	if c := s.Last(); c.Method != "GET" || c.Path != "/channels/c/messages/m1" {
		t.Errorf("call = %s %s", c.Method, c.Path)
	}
}

func TestMessagesGetUsesTheDefaultChannel(t *testing.T) {
	s := transport.NewStub().Reply(`{"id":"m1"}`)
	if _, err := msgs(s, "def").Get(context.Background(), "", "m1"); err != nil {
		t.Fatal(err)
	}
	if c := s.Last(); c.Path != "/channels/def/messages/m1" {
		t.Errorf("path = %s", c.Path)
	}
}

// A webhook-driven channel carries its content in embeds, not in `content`. A
// message decoded without them reads as empty, which is how a bot's report
// becomes invisible to anything reading the channel.
func TestMessageDecodesEmbeds(t *testing.T) {
	s := transport.NewStub().Reply(`{"id":"m1","content":"","embeds":[{
		"title":"Quest failed","description":"stack trace here","url":"https://example.test/run/1",
		"image":{"url":"https://cdn.example.test/shot.png","proxy_url":"https://media.discordapp.net/external/shot.png"},
		"thumbnail":{"url":"https://cdn.example.test/thumb.png"},
		"fields":[{"name":"env","value":"prod"}]}]}`)
	got, err := msgs(s, "def").Get(context.Background(), "c", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Embeds) != 1 {
		t.Fatalf("embeds = %+v", got.Embeds)
	}
	e := got.Embeds[0]
	if e.Title != "Quest failed" || e.Description != "stack trace here" || e.URL != "https://example.test/run/1" {
		t.Errorf("embed = %+v", e)
	}
	if e.Image == nil || e.Image.URL != "https://cdn.example.test/shot.png" {
		t.Errorf("image = %+v", e.Image)
	}
	if e.Image.ProxyURL != "https://media.discordapp.net/external/shot.png" {
		t.Errorf("proxy url = %q", e.Image.ProxyURL)
	}
	if e.Thumbnail == nil || e.Thumbnail.URL != "https://cdn.example.test/thumb.png" {
		t.Errorf("thumbnail = %+v", e.Thumbnail)
	}
	if len(e.Fields) != 1 || e.Fields[0].Name != "env" || e.Fields[0].Value != "prod" {
		t.Errorf("fields = %+v", e.Fields)
	}
}

// An embed with no image must decode to a nil pointer rather than an empty
// struct, so a reader can tell "no picture" from "a picture at the empty url".
func TestEmbedWithoutImageDecodesNil(t *testing.T) {
	s := transport.NewStub().Reply(`{"id":"m1","embeds":[{"title":"no picture"}]}`)
	got, err := msgs(s, "def").Get(context.Background(), "c", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Embeds[0].Image != nil || got.Embeds[0].Thumbnail != nil {
		t.Fatalf("embed = %+v, want nil media", got.Embeds[0])
	}
}

func TestMessagesDelete(t *testing.T) {
	s := transport.NewStub()
	if err := msgs(s, "").Delete(context.Background(), "c", "m1"); err != nil {
		t.Fatal(err)
	}
	if c := s.Last(); c.Method != "DELETE" || c.Path != "/channels/c/messages/m1" {
		t.Errorf("call = %s %s", c.Method, c.Path)
	}
}
