package dctl

import (
	"context"
	"testing"

	"github.com/Herrscherd/dctl/internal/transport"
)

func thr(s *transport.Stub) *Threads {
	return &Threads{rt: s, def: &defaults{guilds: &Guilds{rt: s}}}
}

func TestThreadsStart(t *testing.T) {
	s := transport.NewStub().Reply(`{"id":"t1","name":"topic","type":11}`)
	ch, err := thr(s).Start(context.Background(), "c", "m", "topic")
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID != "t1" {
		t.Fatalf("thread = %+v", ch)
	}
	c := s.Last()
	if c.Path != "/channels/c/messages/m/threads" {
		t.Errorf("path = %s", c.Path)
	}
	body := c.Body.(map[string]any)
	if body["name"] != "topic" {
		t.Errorf("name = %v", body["name"])
	}
	if body["auto_archive_duration"] != autoArchive {
		t.Errorf("auto_archive_duration = %v", body["auto_archive_duration"])
	}
}

// A private thread is created off the CHANNEL, never off a message: a message
// would be the public trace the thread exists to avoid.
func TestThreadsStartPrivate(t *testing.T) {
	s := transport.NewStub().Reply(`{"id":"t9","name":"job","type":12}`)
	ch, err := thr(s).StartPrivate(context.Background(), "c1", "job")
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID != "t9" || ch.Type != ChannelPrivateThread {
		t.Fatalf("thread = %+v", ch)
	}
	c := s.Last()
	if c.Method != "POST" || c.Path != "/channels/c1/threads" {
		t.Fatalf("call = %s %s", c.Method, c.Path)
	}
	body := c.Body.(map[string]any)
	if body["type"] != ChannelPrivateThread {
		t.Errorf("type = %v, want a private thread", body["type"])
	}
	// invitable would let any member drag others in, which defeats the point.
	if body["invitable"] != false {
		t.Errorf("invitable = %v, want false", body["invitable"])
	}
	if body["auto_archive_duration"] != autoArchive {
		t.Errorf("auto_archive_duration = %v", body["auto_archive_duration"])
	}
}

func TestThreadsAddMember(t *testing.T) {
	s := transport.NewStub().Reply(``)
	if err := thr(s).AddMember(context.Background(), "t9", "u1"); err != nil {
		t.Fatal(err)
	}
	c := s.Last()
	if c.Method != "PUT" || c.Path != "/channels/t9/thread-members/u1" {
		t.Fatalf("call = %s %s", c.Method, c.Path)
	}
}

func TestThreadsCreateForum(t *testing.T) {
	s := transport.NewStub().
		Reply(`[{"id":"g1"}]`).
		Reply(`{"id":"f1","name":"forum","type":15}`)
	ch, err := thr(s).CreateForum(context.Background(), "", "forum")
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID != "f1" {
		t.Fatalf("channel = %+v", ch)
	}
	c := s.Last()
	if c.Method != "POST" || c.Path != "/guilds/g1/channels" {
		t.Errorf("call = %s %s", c.Method, c.Path)
	}
	body := c.Body.(map[string]any)
	if body["type"] != ChannelForum {
		t.Errorf("type = %v", body["type"])
	}
}

func TestThreadsForumPost(t *testing.T) {
	s := transport.NewStub().Reply(`{"id":"p1","type":11}`)
	ch, err := thr(s).ForumPost(context.Background(), "f1", "title", "body")
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID != "p1" {
		t.Fatalf("thread = %+v", ch)
	}
	c := s.Last()
	if c.Path != "/channels/f1/threads" {
		t.Errorf("path = %s", c.Path)
	}
	body := c.Body.(map[string]any)
	msg := body["message"].(map[string]any)
	if msg["content"] != "body" {
		t.Errorf("message.content = %v", msg["content"])
	}
}
