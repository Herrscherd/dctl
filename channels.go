package dctl

import (
	"context"
	"net/http"
	"strings"

	"github.com/Herrscherd/dctl/internal/transport"
)

// Channel type constants.
const (
	ChannelText          = 0
	ChannelCategory      = 4
	ChannelNewsThread    = 10
	ChannelPublicThread  = 11
	ChannelPrivateThread = 12
	ChannelForum         = 15
)

// Channels CRUDs guild channels.
type Channels struct {
	rt  transport.Doer
	def *defaults
}

// List returns the channels of a guild (or the sole guild when guildID is empty).
//
// It returns [ErrNoGuild] or an [*AmbiguousGuildError] when no server can be chosen.
func (c *Channels) List(ctx context.Context, guildID string) ([]Channel, error) {
	gid, err := c.def.resolveGuild(ctx, guildID)
	if err != nil {
		return nil, err
	}
	var chs []Channel
	if err := c.rt.Do(ctx, http.MethodGet, "/guilds/"+seg(gid)+"/channels", nil, &chs); err != nil {
		return nil, err
	}
	return chs, nil
}

// Get returns the channel with the given ID.
func (c *Channels) Get(ctx context.Context, channelID string) (*Channel, error) {
	var channel Channel
	if err := c.rt.Do(ctx, http.MethodGet, "/channels/"+seg(channelID), nil, &channel); err != nil {
		return nil, err
	}
	return &channel, nil
}

// Type returns the Discord channel-type integer for channelID.
func (c *Channels) Type(ctx context.Context, channelID string) (int, error) {
	channel, err := c.Get(ctx, channelID)
	if err != nil {
		return 0, err
	}
	return channel.Type, nil
}

// Create creates a text channel named name in guildID (or the sole guild).
func (c *Channels) Create(ctx context.Context, guildID, name string) (*Channel, error) {
	return c.create(ctx, guildID, map[string]any{fieldName: name, fieldType: ChannelText})
}

// CreateUnder creates a text channel nested under category parentID, in that
// category's own guild.
func (c *Channels) CreateUnder(ctx context.Context, parentID, name string) (*Channel, error) {
	gid, err := c.guildOf(ctx, parentID)
	if err != nil {
		return nil, err
	}
	return c.create(ctx, gid, map[string]any{fieldName: name, fieldType: ChannelText, "parent_id": parentID})
}

func (c *Channels) guildOf(ctx context.Context, channelID string) (string, error) {
	if channelID == "" {
		return "", nil
	}
	channel, err := c.Get(ctx, channelID)
	if err != nil {
		return "", err
	}
	return channel.GuildID, nil
}

func (c *Channels) create(ctx context.Context, guildID string, body map[string]any) (*Channel, error) {
	gid, err := c.def.resolveGuild(ctx, guildID)
	if err != nil {
		return nil, err
	}
	var channel Channel
	if err := c.rt.Do(ctx, http.MethodPost, "/guilds/"+seg(gid)+"/channels", body, &channel); err != nil {
		return nil, err
	}
	return &channel, nil
}

// Rename sets the name of a channel and returns the updated channel.
func (c *Channels) Rename(ctx context.Context, channelID, name string) (*Channel, error) {
	return c.Update(ctx, channelID, map[string]any{fieldName: name})
}

// Update PATCHes arbitrary channel fields (name, parent_id, topic, position…).
func (c *Channels) Update(ctx context.Context, channelID string, fields map[string]any) (*Channel, error) {
	var channel Channel
	if err := c.rt.Do(ctx, http.MethodPatch, "/channels/"+seg(channelID), fields, &channel); err != nil {
		return nil, err
	}
	return &channel, nil
}

// Delete removes a channel. It returns [ErrNoChannel] when channelID is empty.
func (c *Channels) Delete(ctx context.Context, channelID string) error {
	if channelID == "" {
		return ErrNoChannel
	}
	return c.rt.Do(ctx, http.MethodDelete, "/channels/"+seg(channelID), nil, nil)
}

// Ensure returns the text channel named name in the guild, creating it if absent.
// Matching is case-insensitive.
func (c *Channels) Ensure(ctx context.Context, guildID, name string) (*Channel, error) {
	chs, err := c.List(ctx, guildID)
	if err != nil {
		return nil, err
	}
	for index := range chs {
		if chs[index].Type == ChannelText && strings.EqualFold(chs[index].Name, name) {
			return &chs[index], nil
		}
	}
	return c.Create(ctx, guildID, name)
}

// EnsureUnder returns the text channel named name nested under category parentID,
// creating it there if absent. Matching is case-insensitive and scoped to the
// parent so the same name can exist under different categories.
func (c *Channels) EnsureUnder(ctx context.Context, parentID, name string) (*Channel, error) {
	gid, err := c.guildOf(ctx, parentID)
	if err != nil {
		return nil, err
	}
	chs, err := c.List(ctx, gid)
	if err != nil {
		return nil, err
	}
	for index := range chs {
		if chs[index].Type == ChannelText && chs[index].ParentID == parentID && strings.EqualFold(chs[index].Name, name) {
			return &chs[index], nil
		}
	}
	return c.create(ctx, gid, map[string]any{fieldName: name, fieldType: ChannelText, "parent_id": parentID})
}

// Archive archives a thread/forum-post, or deletes a plain text channel
// (text channels don't support PATCH {archived:true}).
//
// It fails when the channel type cannot be read, or when Discord rejects the request.
func (c *Channels) Archive(ctx context.Context, channelID string) error {
	channelType, err := c.Type(ctx, channelID)
	if err != nil {
		return err
	}
	if channelType == ChannelNewsThread || channelType == ChannelPublicThread || channelType == ChannelPrivateThread {
		return c.rt.Do(ctx, http.MethodPatch, "/channels/"+seg(channelID), map[string]any{"archived": true}, nil)
	}
	return c.Delete(ctx, channelID)
}
