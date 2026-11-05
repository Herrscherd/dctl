package dctl

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/Herrscherd/dctl/internal/transport"
)

// Interaction type constants we care about (Discord interaction-type field).
const (
	InteractionCommand      = 2
	InteractionComponent    = 3
	InteractionAutocomplete = 4
)

// Interaction is the subset of a Discord INTERACTION_CREATE we handle
// (application slash commands, type 2; autocomplete requests, type 4).
type Interaction struct {
	ID          string          `json:"id"`
	Type        int             `json:"type"`
	Token       Secret          `json:"token"`
	GuildID     string          `json:"guild_id"`
	ChannelID   string          `json:"channel_id"`
	Locale      string          `json:"locale"`
	GuildLocale string          `json:"guild_locale"`
	Member      Member          `json:"member"`
	User        Author          `json:"user"`
	Data        InteractionData `json:"data"`
}

// Member carries the invoking user (interactions in a guild come via member.user).
type Member struct {
	User Author `json:"user"`
}

// UserID is who invoked the interaction, wherever Discord put them: a guild
// interaction carries the caller under `member.user`, a DM under `user`. Reading
// only one of the two makes every permission check in the other context see an
// empty id.
func (i Interaction) UserID() string {
	if id := i.Member.User.ID; id != "" {
		return id
	}
	return i.User.ID
}

// InteractionData is the invoked command + its options. For a component
// interaction (type 3) the command fields are empty and CustomID/Values carry
// the clicked component's id and the selected value(s) instead.
type InteractionData struct {
	Name     string              `json:"name"`
	Options  []InteractionOption `json:"options"`
	CustomID string              `json:"custom_id"`
	Values   []string            `json:"values"`
}

// InteractionOption is one command option; for subcommands, Options nests.
// Focused is set on the single option the user is currently typing in an
// autocomplete interaction.
type InteractionOption struct {
	Name    string              `json:"name"`
	Type    int                 `json:"type"`
	Value   any                 `json:"value"`
	Focused bool                `json:"focused"`
	Options []InteractionOption `json:"options"`
}

// Response is what the Handler decides to reply with.
type Response struct {
	Content   string
	Ephemeral bool
}

// AutocompleteChoice is one suggestion returned for an autocomplete interaction.
// Name is shown in the picker; Value is what gets submitted.
type AutocompleteChoice struct {
	Name  string
	Value string
}

// Opt returns the string value of a (possibly nested) option by name.
func (d InteractionData) Opt(name string) (string, bool) {
	return findOpt(d.Options, name)
}

func findOpt(opts []InteractionOption, name string) (string, bool) {
	for _, option := range opts {
		if option.Name == name {
			if s, ok := option.Value.(string); ok {
				return s, true
			}
		}
		if v, ok := findOpt(option.Options, name); ok {
			return v, true
		}
	}
	return "", false
}

// OptBool returns the bool value of a (possibly nested) option, false if absent.
func (d InteractionData) OptBool(name string) bool {
	if b, ok := findTyped[bool](d.Options, name); ok {
		return b
	}
	return false
}

func findTyped[Value any](opts []InteractionOption, name string) (Value, bool) {
	for _, option := range opts {
		if option.Name == name {
			if v, ok := option.Value.(Value); ok {
				return v, true
			}
		}
		if v, ok := findTyped[Value](option.Options, name); ok {
			return v, true
		}
	}
	var zero Value
	return zero, false
}

// OptInt returns the integer value of a (possibly nested) option by name. Discord
// delivers numbers as JSON floats, so INTEGER options arrive as float64 here.
func (d InteractionData) OptInt(name string) (int64, bool) {
	if v, ok := findValue(d.Options, name); ok {
		if f, ok := v.(float64); ok {
			return int64(f), true
		}
	}
	return 0, false
}

// OptFloat returns the float value of a (possibly nested) NUMBER option by name.
func (d InteractionData) OptFloat(name string) (float64, bool) {
	if v, ok := findValue(d.Options, name); ok {
		if f, ok := v.(float64); ok {
			return f, true
		}
	}
	return 0, false
}

func findValue(opts []InteractionOption, name string) (any, bool) {
	for _, option := range opts {
		if option.Name == name && option.Value != nil {
			return option.Value, true
		}
		if v, ok := findValue(option.Options, name); ok {
			return v, true
		}
	}
	return nil, false
}

// Focused returns the name and current (partial) string value of the option the
// user is typing in an autocomplete interaction, searching nested subcommands.
func (d InteractionData) Focused() (string, string, bool) {
	focus, ok := findFocused(d.Options)
	return focus.name, focus.text, ok
}

type focusedOption struct {
	name string
	text string
}

func findFocused(opts []InteractionOption) (focusedOption, bool) {
	for _, option := range opts {
		if option.Focused {
			text, isText := option.Value.(string)
			if !isText {
				text = ""
			}
			return focusedOption{name: option.Name, text: text}, true
		}
		if focus, ok := findFocused(option.Options); ok {
			return focus, true
		}
	}
	return focusedOption{}, false
}

// Subcommand returns the name of the first sub-command option, if any.
func (d InteractionData) Subcommand() (string, []InteractionOption) {
	for _, option := range d.Options {
		if option.Type == optSubCommand {
			return option.Name, option.Options
		}
	}
	return "", nil
}

// Interactions is the sub-client for Discord interaction endpoints.
type Interactions struct {
	rt  transport.Doer
	def *defaults

	regOnce sync.Once
	reg     *Registry
}

// AppID returns the bot's application id (== bot user id), cached after the
// first /users/@me call.
func (in *Interactions) AppID(ctx context.Context) (string, error) {
	if in.def != nil {
		return in.def.appIDOnce(ctx)
	}
	return fetchAppID(ctx, in.rt)
}

func fetchAppID(ctx context.Context, doer transport.Doer) (string, error) {
	var application struct {
		ID string `json:"id"`
	}
	if err := doer.Do(ctx, http.MethodGet, "/users/@me", nil, &application); err != nil {
		return "", err
	}
	return application.ID, nil
}

// RegisteredCommand is the read form of a registered command.
type RegisteredCommand struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (in *Interactions) commandsBase(ctx context.Context) (string, error) {
	appID, err := in.AppID(ctx)
	if err != nil {
		return "", err
	}
	if in.def.globalCommands {
		return "/applications/" + seg(appID) + "/commands", nil
	}
	gid, err := in.def.resolveGuild(ctx, "")
	if err != nil {
		return "", err
	}
	return "/applications/" + seg(appID) + "/guilds/" + seg(gid) + "/commands", nil
}

// RegisterCommands bulk-overwrites the command scope from raw maps.
func (in *Interactions) RegisterCommands(ctx context.Context, commands []map[string]any) error {
	base, err := in.commandsBase(ctx)
	if err != nil {
		return err
	}
	return in.rt.Do(ctx, http.MethodPut, base, commands, nil)
}

// Register bulk-overwrites the command scope from builders.
func (in *Interactions) Register(ctx context.Context, cmds ...*Command) error {
	body := make([]map[string]any, 0, len(cmds))
	for _, c := range cmds {
		body = append(body, c.build())
	}
	return in.RegisterCommands(ctx, body)
}

// List returns the currently registered commands in the client's scope.
func (in *Interactions) List(ctx context.Context) ([]RegisteredCommand, error) {
	base, err := in.commandsBase(ctx)
	if err != nil {
		return nil, err
	}
	var out []RegisteredCommand
	if err := in.rt.Do(ctx, http.MethodGet, base, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Create registers one command and returns its server-assigned id.
func (in *Interactions) Create(ctx context.Context, cmd *Command) (RegisteredCommand, error) {
	base, err := in.commandsBase(ctx)
	if err != nil {
		return RegisteredCommand{}, err
	}
	var out RegisteredCommand
	if err := in.rt.Do(ctx, http.MethodPost, base, cmd.build(), &out); err != nil {
		return RegisteredCommand{}, err
	}
	return out, nil
}

// Edit updates the command with the given id.
func (in *Interactions) Edit(ctx context.Context, commandID string, cmd *Command) error {
	base, err := in.commandsBase(ctx)
	if err != nil {
		return err
	}
	return in.rt.Do(ctx, http.MethodPatch, base+"/"+seg(commandID), cmd.build(), nil)
}

// Delete removes the command with the given id.
func (in *Interactions) Delete(ctx context.Context, commandID string) error {
	base, err := in.commandsBase(ctx)
	if err != nil {
		return err
	}
	return in.rt.Do(ctx, http.MethodDelete, base+"/"+seg(commandID), nil, nil)
}

// Registry returns the command registry bound to these interactions. The same
// registry is returned on every call, so bindings registered at setup are seen
// at dispatch time.
func (in *Interactions) Registry() *Registry {
	in.regOnce.Do(func() {
		in.reg = &Registry{in: in, entries: make(map[string]regEntry)}
	})
	return in.reg
}

// Respond sends a CHANNEL_MESSAGE_WITH_SOURCE (type 4) reply.
func (in *Interactions) Respond(ctx context.Context, interactionID, token string, response Response) error {
	data := map[string]any{fieldContent: response.Content}
	if response.Ephemeral {
		data["flags"] = flagEphemeral
	}
	return in.rt.Do(ctx, http.MethodPost, "/interactions/"+seg(interactionID)+"/"+seg(token)+"/callback",
		map[string]any{fieldType: callbackChannelMessage, fieldData: data}, nil)
}

// Defer acknowledges an interaction with a DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE
// (type 5). The ephemeral flag must match the eventual reply's visibility.
func (in *Interactions) Defer(ctx context.Context, interactionID, token string, ephemeral bool) error {
	data := make(map[string]any)
	if ephemeral {
		data["flags"] = flagEphemeral
	}
	return in.rt.Do(ctx, http.MethodPost, "/interactions/"+seg(interactionID)+"/"+seg(token)+"/callback",
		map[string]any{fieldType: callbackDeferredMessage, fieldData: data}, nil)
}

// RespondAutocomplete sends an APPLICATION_COMMAND_AUTOCOMPLETE_RESULT (type 8)
// reply carrying the suggestion list. Discord accepts at most 25 choices; extras
// are dropped here so callers need not pre-trim.
func (in *Interactions) RespondAutocomplete(ctx context.Context, interactionID, token string, choices []AutocompleteChoice) error {
	if len(choices) > maxAutocompleteChoices {
		choices = choices[:maxAutocompleteChoices]
	}
	encoded := make([]map[string]any, 0, len(choices))
	for _, ch := range choices {
		encoded = append(encoded, map[string]any{fieldName: ch.Name, fieldValue: ch.Value})
	}
	return in.rt.Do(ctx, http.MethodPost, "/interactions/"+seg(interactionID)+"/"+seg(token)+"/callback",
		map[string]any{fieldType: callbackAutocompleteResult, fieldData: map[string]any{"choices": encoded}}, nil)
}

// EditResponse fills in the deferred reply by editing the original interaction
// response via the webhook endpoint. appID is the bot's application id (see
// AppID); the interaction token authorizes the edit for ~15 minutes.
func (in *Interactions) EditResponse(ctx context.Context, appID, token string, response Response) error {
	return in.rt.Do(ctx, http.MethodPatch, "/webhooks/"+seg(appID)+"/"+seg(token)+"/messages/@original",
		map[string]any{fieldContent: response.Content}, nil)
}

// UpsertStatusMessage edits the existing status message or sends a new one,
// returning the live message id.
func (in *Interactions) UpsertStatusMessage(ctx context.Context, channelID, msgID, content string) (string, error) {
	msgs := &Messages{rt: in.rt, def: in.def}
	if msgID != "" {
		_, err := msgs.Edit(ctx, channelID, msgID, content)
		if err == nil {
			return msgID, nil
		}
		var apiErr *transport.APIError
		if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
			return "", err
		}
	}
	sent, err := msgs.Send(ctx, channelID, content)
	if err != nil {
		return "", err
	}
	return sent.ID, nil
}
