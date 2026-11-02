package dctl

import (
	"context"
	"net/http"
	"sync"
)

// Handler runs a command interaction and returns the reply.
type Handler func(ctx context.Context, ix Interaction) (Response, error)

// AutocompleteHandler returns the suggestions for an autocomplete interaction.
type AutocompleteHandler func(ctx context.Context, ix Interaction) ([]AutocompleteChoice, error)

type regEntry struct {
	cmd          *Command
	handler      Handler
	autocomplete AutocompleteHandler
}

// Registry holds name→{command, handler} bindings, syncs them to Discord
// (add/remove/update) and dispatches incoming interactions to their handler.
type Registry struct {
	in      *Interactions
	mu      sync.Mutex
	order   []string
	entries map[string]regEntry
}

// Add binds a command to a handler, replacing any existing binding by name.
func (r *Registry) Add(cmd *Command, handler Handler) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := cmd.Name()
	entry := r.entries[name]
	if entry.cmd == nil {
		r.order = append(r.order, name)
	}
	entry.cmd, entry.handler = cmd, handler
	r.entries[name] = entry
	return r
}

// Update is an alias for Add, for call sites that mean "replace".
func (r *Registry) Update(cmd *Command, h Handler) *Registry { return r.Add(cmd, h) }

// Autocomplete attaches an autocomplete handler to an already-added command.
func (r *Registry) Autocomplete(name string, handler AutocompleteHandler) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.entries[name]; ok {
		entry.autocomplete = handler
		r.entries[name] = entry
	}
	return r
}

// Remove drops a binding by command name.
func (r *Registry) Remove(name string) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, name)
	for index, n := range r.order {
		if n == name {
			r.order = append(r.order[:index], r.order[index+1:]...)
			break
		}
	}
	return r
}

// Commands returns the bound commands in insertion order.
func (r *Registry) Commands() []*Command {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Command, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, r.entries[n].cmd)
	}
	return out
}

type desiredCmd struct {
	name string
	body map[string]any
}

func (r *Registry) snapshot() []desiredCmd {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]desiredCmd, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, desiredCmd{name: n, body: r.entries[n].cmd.build()})
	}
	return out
}

// Sync reconciles Discord with the registry. It creates new commands, edits
// existing ones and deletes those no longer bound.
//
// Sync refuses to run on an empty registry while commands exist. It returns a
// [*SyncWouldDeleteError] in that case, so no command is deleted by accident.
func (r *Registry) Sync(ctx context.Context) error {
	base, err := r.in.commandsBase(ctx)
	if err != nil {
		return err
	}
	var existing []RegisteredCommand
	if err := r.in.rt.Do(ctx, http.MethodGet, base, nil, &existing); err != nil {
		return err
	}

	desired := r.snapshot()
	if len(desired) == 0 && len(existing) > 0 {
		return &SyncWouldDeleteError{Count: len(existing)}
	}

	if err := r.upsertCommands(ctx, base, desired, existing); err != nil {
		return err
	}
	keep := make(map[string]bool, len(desired))
	for _, d := range desired {
		keep[d.name] = true
	}
	for _, existingCommand := range existing {
		if !keep[existingCommand.Name] {
			if err := r.in.rt.Do(ctx, http.MethodDelete, base+"/"+seg(existingCommand.ID), nil, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Registry) upsertCommands(ctx context.Context, base string, desired []desiredCmd, existing []RegisteredCommand) error {
	byName := make(map[string]RegisteredCommand, len(existing))
	for _, c := range existing {
		byName[c.Name] = c
	}
	for _, wantedCommand := range desired {
		method, path := http.MethodPost, base
		if cur, ok := byName[wantedCommand.name]; ok {
			method, path = http.MethodPatch, base+"/"+seg(cur.ID)
		}
		if err := r.in.rt.Do(ctx, method, path, wantedCommand.body, nil); err != nil {
			return err
		}
	}
	return nil
}

// Dispatch routes a command interaction to its handler by command name.
// Autocomplete interactions must go through DispatchAutocomplete instead.
func (r *Registry) Dispatch(ctx context.Context, interaction Interaction) (Response, error) {
	if interaction.Type == InteractionAutocomplete {
		return Response{}, &AutocompleteInteractionError{Name: interaction.Data.Name}
	}
	r.mu.Lock()
	entry, ok := r.entries[interaction.Data.Name]
	r.mu.Unlock()
	if !ok {
		return Response{}, &UnknownCommandError{Name: interaction.Data.Name}
	}
	return entry.handler(ctx, interaction)
}

// DispatchAutocomplete routes an autocomplete interaction to its autocomplete
// handler by command name.
func (r *Registry) DispatchAutocomplete(ctx context.Context, interaction Interaction) ([]AutocompleteChoice, error) {
	r.mu.Lock()
	entry, ok := r.entries[interaction.Data.Name]
	r.mu.Unlock()
	if !ok || entry.autocomplete == nil {
		return nil, &NoAutocompleteHandlerError{Name: interaction.Data.Name}
	}
	return entry.autocomplete(ctx, interaction)
}
