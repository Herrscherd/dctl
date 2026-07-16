package dctl

import (
	"context"
	"errors"
	"sync"

	"github.com/Herrscherd/dctl/internal/transport"
)

// ErrNoChannel is returned when neither an explicit channel nor a default is set.
var ErrNoChannel = errors.New("dctl: no channel (DISCORD_CHANNEL_ID or --channel)")

// defaults resolves and caches the default channel/guild/app-id shared across
// sub-clients. The bot's application id and sole-guild id are immutable for the
// client's lifetime, so they are fetched once and memoized.
type defaults struct {
	rt      transport.Doer
	channel string
	guilds  *Guilds

	// appID and the sole-guild id are resolved independently via one network
	// call each; separate locks keep a slow guild lookup from blocking an app-id
	// lookup (and vice-versa). Each guards only its own value, held across the
	// fetch so concurrent callers of the same value dedupe rather than stampede.
	muApp    sync.Mutex
	appID    string
	muGuild  sync.Mutex
	soleGuid string
}

func (d *defaults) resolveChannel(channelID string) (string, error) {
	if channelID != "" {
		return channelID, nil
	}
	if d.channel == "" {
		return "", ErrNoChannel
	}
	return d.channel, nil
}

func (d *defaults) resolveGuild(ctx context.Context, guildID string) (string, error) {
	if guildID != "" {
		return guildID, nil
	}
	d.muGuild.Lock()
	defer d.muGuild.Unlock()
	if d.soleGuid != "" {
		return d.soleGuid, nil
	}
	g, err := d.guilds.Sole(ctx)
	if err != nil {
		return "", err
	}
	d.soleGuid = g.ID
	return d.soleGuid, nil
}

func (d *defaults) appIDOnce(ctx context.Context) (string, error) {
	d.muApp.Lock()
	defer d.muApp.Unlock()
	if d.appID != "" {
		return d.appID, nil
	}
	id, err := fetchAppID(ctx, d.rt)
	if err != nil {
		return "", err
	}
	d.appID = id
	return id, nil
}
