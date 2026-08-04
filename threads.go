package dctl

import (
	"context"
	"net/http"

	"github.com/Herrscherd/dctl/internal/transport"
)

// Threads creates threads and forum posts.
type Threads struct {
	rt  transport.Doer
	def *defaults
}

// Start opens a public thread off messageID in channelID.
// auto_archive_duration is set to 1440 minutes (24 h).
func (t *Threads) Start(ctx context.Context, channelID, messageID, name string) (*Channel, error) {
	var ch Channel
	if err := t.rt.Do(ctx, http.MethodPost, "/channels/"+seg(channelID)+"/messages/"+seg(messageID)+"/threads",
		map[string]any{"name": name, "auto_archive_duration": autoArchive}, &ch); err != nil {
		return nil, err
	}
	return &ch, nil
}

// StartPrivate opens a private thread in channelID, with no parent message so
// the channel shows no trace of it. Only invited members and moderators holding
// MANAGE_THREADS can see it — invitable is false, so members cannot pull others
// in. The creating bot is a member; add the humans with AddMember.
func (t *Threads) StartPrivate(ctx context.Context, channelID, name string) (*Channel, error) {
	var ch Channel
	if err := t.rt.Do(ctx, http.MethodPost, "/channels/"+seg(channelID)+"/threads",
		map[string]any{
			"name":                  name,
			"type":                  ChannelPrivateThread,
			"invitable":             false,
			"auto_archive_duration": autoArchive,
		}, &ch); err != nil {
		return nil, err
	}
	return &ch, nil
}

// AddMember gives userID access to threadID. A private thread is invisible until
// its members are added, so this is what makes one usable by a person.
func (t *Threads) AddMember(ctx context.Context, threadID, userID string) error {
	return t.rt.Do(ctx, http.MethodPut,
		"/channels/"+seg(threadID)+"/thread-members/"+seg(userID), nil, nil)
}

// CreateForum creates a forum channel named name in guildID (or the sole guild).
func (t *Threads) CreateForum(ctx context.Context, guildID, name string) (*Channel, error) {
	gid, err := t.def.resolveGuild(ctx, guildID)
	if err != nil {
		return nil, err
	}
	var ch Channel
	if err := t.rt.Do(ctx, http.MethodPost, "/guilds/"+seg(gid)+"/channels",
		map[string]any{"name": name, "type": ChannelForum}, &ch); err != nil {
		return nil, err
	}
	return &ch, nil
}

// ForumPost creates a thread (post) in forum forumID with an initial message.
func (t *Threads) ForumPost(ctx context.Context, forumID, name, content string) (*Channel, error) {
	var ch Channel
	if err := t.rt.Do(ctx, http.MethodPost, "/channels/"+seg(forumID)+"/threads",
		map[string]any{"name": name, "message": map[string]any{"content": content}}, &ch); err != nil {
		return nil, err
	}
	return &ch, nil
}

const autoArchive = 1440
