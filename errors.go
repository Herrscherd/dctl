package dctl

import (
	"errors"
	"fmt"
)

// ErrNoGuild is returned by [Guilds.Sole] when the bot is in no server.
var ErrNoGuild = errors.New("dctl: bot is in no server (invite it first)")

// AmbiguousGuildError is returned by [Guilds.Sole] when the bot is in more
// than one server, so no single one can be chosen. Count is the number of
// servers the bot is in.
type AmbiguousGuildError struct {
	Count int
}

func (e *AmbiguousGuildError) Error() string {
	return fmt.Sprintf("dctl: bot is in %d servers; pass a guild id", e.Count)
}

// SyncWouldDeleteError is returned by [Registry.Sync] when no command is
// registered in code but Discord still holds Count commands, so a sync would
// delete them all.
type SyncWouldDeleteError struct {
	Count int
}

func (e *SyncWouldDeleteError) Error() string {
	return fmt.Sprintf("dctl: Sync on empty registry would delete %d command(s); add commands first", e.Count)
}

// AutocompleteInteractionError is returned by [Registry.Dispatch] when the
// interaction is an autocomplete request, which only [Registry.DispatchAutocomplete]
// answers. Name is the command name of the interaction.
type AutocompleteInteractionError struct {
	Name string
}

func (e *AutocompleteInteractionError) Error() string {
	return fmt.Sprintf("dctl: autocomplete interaction for %q; use DispatchAutocomplete", e.Name)
}

// UnknownCommandError is returned by [Registry.Dispatch] when no command is
// registered under the name of the interaction. Name is that command name.
type UnknownCommandError struct {
	Name string
}

func (e *UnknownCommandError) Error() string {
	return fmt.Sprintf("dctl: unknown command %q", e.Name)
}

// NoAutocompleteHandlerError is returned by [Registry.DispatchAutocomplete]
// when the command has no autocomplete handler. Name is the command name.
type NoAutocompleteHandlerError struct {
	Name string
}

func (e *NoAutocompleteHandlerError) Error() string {
	return fmt.Sprintf("dctl: no autocomplete handler for %q", e.Name)
}
