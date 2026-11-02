package dctl

import (
	"context"
	"net/http"

	"github.com/Herrscherd/dctl/internal/transport"
)

// Guilds lists and resolves the bot's servers.
type Guilds struct {
	rt transport.Doer
}

// List returns the servers the bot is a member of.
func (g *Guilds) List(ctx context.Context) ([]Guild, error) {
	var guilds []Guild
	if err := g.rt.Do(ctx, http.MethodGet, "/users/@me/guilds", nil, &guilds); err != nil {
		return nil, err
	}
	return guilds, nil
}

// Sole resolves the bot's single server (mono-server). Errors if the bot is in
// zero or several guilds, so callers never silently target the wrong one.
func (g *Guilds) Sole(ctx context.Context) (Guild, error) {
	guilds, err := g.List(ctx)
	if err != nil {
		return Guild{}, err
	}
	switch len(guilds) {
	case 0:
		return Guild{}, ErrNoGuild
	case 1:
		return guilds[0], nil
	default:
		return Guild{}, &AmbiguousGuildError{Count: len(guilds)}
	}
}
