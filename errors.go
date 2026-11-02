package dctl

import (
	"errors"
	"fmt"
)

var ErrNoGuild = errors.New("dctl: bot is in no server (invite it first)")

type AmbiguousGuildError struct {
	Count int
}

func (e *AmbiguousGuildError) Error() string {
	return fmt.Sprintf("dctl: bot is in %d servers; pass a guild id", e.Count)
}

type SyncWouldDeleteError struct {
	Count int
}

func (e *SyncWouldDeleteError) Error() string {
	return fmt.Sprintf("dctl: Sync on empty registry would delete %d command(s); add commands first", e.Count)
}

type AutocompleteInteractionError struct {
	Name string
}

func (e *AutocompleteInteractionError) Error() string {
	return fmt.Sprintf("dctl: autocomplete interaction for %q; use DispatchAutocomplete", e.Name)
}

type UnknownCommandError struct {
	Name string
}

func (e *UnknownCommandError) Error() string {
	return fmt.Sprintf("dctl: unknown command %q", e.Name)
}

type NoAutocompleteHandlerError struct {
	Name string
}

func (e *NoAutocompleteHandlerError) Error() string {
	return fmt.Sprintf("dctl: no autocomplete handler for %q", e.Name)
}
