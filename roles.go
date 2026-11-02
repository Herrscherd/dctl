package dctl

import (
	"context"
	"net/http"

	"github.com/Herrscherd/dctl/internal/transport"
)

// Roles CRUDs guild roles and assigns them to members.
type Roles struct {
	rt  transport.Doer
	def *defaults
}

// List returns the roles of guildID (or the sole guild when empty).
func (r *Roles) List(ctx context.Context, guildID string) ([]Role, error) {
	gid, err := r.def.resolveGuild(ctx, guildID)
	if err != nil {
		return nil, err
	}
	var roles []Role
	if err := r.rt.Do(ctx, http.MethodGet, "/guilds/"+seg(gid)+"/roles", nil, &roles); err != nil {
		return nil, err
	}
	return roles, nil
}

// Create creates a role named name in guildID (or the sole guild).
func (r *Roles) Create(ctx context.Context, guildID, name string) (*Role, error) {
	gid, err := r.def.resolveGuild(ctx, guildID)
	if err != nil {
		return nil, err
	}
	var role Role
	if err := r.rt.Do(ctx, http.MethodPost, "/guilds/"+seg(gid)+"/roles",
		map[string]any{fieldName: name}, &role); err != nil {
		return nil, err
	}
	return &role, nil
}

// Update PATCHes role fields (name, color, permissions…).
func (r *Roles) Update(ctx context.Context, guildID, roleID string, fields map[string]any) (*Role, error) {
	gid, err := r.def.resolveGuild(ctx, guildID)
	if err != nil {
		return nil, err
	}
	var role Role
	if err := r.rt.Do(ctx, http.MethodPatch, "/guilds/"+seg(gid)+"/roles/"+seg(roleID), fields, &role); err != nil {
		return nil, err
	}
	return &role, nil
}

// Delete removes a role from a server. The server is resolved as in [Members.Get].
//
// It returns [ErrNoGuild] or an [*AmbiguousGuildError] when no server can be chosen. It fails when Discord rejects the request.
func (r *Roles) Delete(ctx context.Context, guildID, roleID string) error {
	gid, err := r.def.resolveGuild(ctx, guildID)
	if err != nil {
		return err
	}
	return r.rt.Do(ctx, http.MethodDelete, "/guilds/"+seg(gid)+"/roles/"+seg(roleID), nil, nil)
}

// Assign grants roleID to member userID.
//
// It returns [ErrNoGuild] or an [*AmbiguousGuildError] when no server can be chosen. It fails when Discord rejects the request.
func (r *Roles) Assign(ctx context.Context, guildID, userID, roleID string) error {
	gid, err := r.def.resolveGuild(ctx, guildID)
	if err != nil {
		return err
	}
	return r.rt.Do(ctx, http.MethodPut, "/guilds/"+seg(gid)+"/members/"+seg(userID)+"/roles/"+seg(roleID), nil, nil)
}

// Unassign removes roleID from member userID.
//
// It returns [ErrNoGuild] or an [*AmbiguousGuildError] when no server can be chosen. It fails when Discord rejects the request.
func (r *Roles) Unassign(ctx context.Context, guildID, userID, roleID string) error {
	gid, err := r.def.resolveGuild(ctx, guildID)
	if err != nil {
		return err
	}
	return r.rt.Do(ctx, http.MethodDelete, "/guilds/"+seg(gid)+"/members/"+seg(userID)+"/roles/"+seg(roleID), nil, nil)
}
