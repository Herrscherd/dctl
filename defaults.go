package dctl

import (
	"context"
	"errors"
	"sync"

	"github.com/Herrscherd/dctl/internal/transport"
)

// ErrNoChannel is returned when neither an explicit channel nor a default is set.
var ErrNoChannel = errors.New("dctl: no channel (DISCORD_CHANNEL_ID or --channel)")

type defaults struct {
	rt      transport.Doer
	channel string
	guild   string
	guilds  *Guilds

	globalCommands bool

	muApp    sync.Mutex
	appID    string
	muGuild  sync.Mutex
	soleGUID string
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
	if d.guild != "" {
		return d.guild, nil
	}
	d.muGuild.Lock()
	defer d.muGuild.Unlock()
	if d.soleGUID != "" {
		return d.soleGUID, nil
	}
	g, err := d.guilds.Sole(ctx)
	if err != nil {
		return "", err
	}
	d.soleGUID = g.ID
	return d.soleGUID, nil
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
