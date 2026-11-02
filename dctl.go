package dctl

import (
	"log/slog"
	"net/http"

	"github.com/Herrscherd/dctl/internal/transport"
)

// Client is the dctl façade: it wires the HTTP transport into per-resource
// sub-clients sharing one default channel/guild resolver. Build it with New.
type Client struct {
	rt           transport.Doer
	def          *defaults
	guilds       *Guilds
	interactions *Interactions
}

// ClientOption configures a Client.
type ClientOption func(*clientConfig)

type clientConfig struct {
	httpClient *http.Client
	logger     *slog.Logger
	guild      string
	global     bool
}

// WithHTTPClient overrides the default 15s-timeout HTTP client.
func WithHTTPClient(h *http.Client) ClientOption {
	return func(c *clientConfig) { c.httpClient = h }
}

// WithLogger sets the logger for the warning raised when a response body fails
// to close after it was read. Without it that warning is discarded.
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *clientConfig) { c.logger = logger }
}

// WithGuild pins the guild that guild-scoped ops target when no explicit id is
// passed. Without it they fall back to Guilds().Sole, which errors when the bot
// is in more than one server. A bot invited to a second guild would otherwise
// lose command registration entirely.
func WithGuild(id string) ClientOption {
	return func(c *clientConfig) { c.guild = id }
}

// WithGlobalCommands registers slash commands on the application rather than on
// one guild, so every server the bot is in gets them. It is the only scope that
// works for a bot meant to be installed anywhere. Guild commands appear
// instantly and global ones take up to an hour to propagate, which is why they
// stay the default. Other guild-scoped ops are unaffected.
func WithGlobalCommands() ClientOption {
	return func(c *clientConfig) { c.global = true }
}

// New builds a Client. token is the bot token (kept in memory only). defaultChannel
// is the channel that message ops target when no explicit channel id is passed.
func New(token, defaultChannel string, opts ...ClientOption) *Client {
	cfg := &clientConfig{}
	for _, o := range opts {
		o(cfg)
	}
	var topts []transport.Option
	if cfg.httpClient != nil {
		topts = append(topts, transport.WithHTTPClient(cfg.httpClient))
	}
	if cfg.logger != nil {
		topts = append(topts, transport.WithLogger(cfg.logger))
	}
	rt := transport.NewHTTP(token, topts...)
	client := newWith(rt, defaultChannel)
	client.def.guild = cfg.guild
	client.def.globalCommands = cfg.global
	return client
}

func newWith(doer transport.Doer, defaultChannel string) *Client {
	guilds := &Guilds{rt: doer}
	def := &defaults{rt: doer, channel: defaultChannel, guilds: guilds}
	return &Client{
		rt:           doer,
		def:          def,
		guilds:       guilds,
		interactions: &Interactions{rt: doer, def: def},
	}
}

// Enabled reports whether the underlying transport is configured.
func (c *Client) Enabled() bool { return c != nil && c.rt.Enabled() }

// DefaultChannel returns the configured default channel id.
func (c *Client) DefaultChannel() string {
	if c == nil {
		return ""
	}
	return c.def.channel
}

// Guilds returns the client for the servers the bot is in.
func (c *Client) Guilds() *Guilds { return c.guilds }

// Messages returns the client for messages.
func (c *Client) Messages() *Messages { return &Messages{rt: c.rt, def: c.def} }

// Channels returns the client for channels.
func (c *Client) Channels() *Channels { return &Channels{rt: c.rt, def: c.def} }

// Roles returns the client for server roles.
func (c *Client) Roles() *Roles { return &Roles{rt: c.rt, def: c.def} }

// Members returns the client for server members.
func (c *Client) Members() *Members { return &Members{rt: c.rt, def: c.def} }

// Reactions returns the client for message reactions.
func (c *Client) Reactions() *Reactions { return &Reactions{rt: c.rt, def: c.def} }

// Threads returns the client for threads.
func (c *Client) Threads() *Threads { return &Threads{rt: c.rt, def: c.def} }

// Permissions returns the client for channel permission overwrites.
func (c *Client) Permissions() *Permissions { return &Permissions{rt: c.rt} }

// Webhooks returns the client for webhooks.
func (c *Client) Webhooks() *Webhooks { return &Webhooks{rt: c.rt} }

// Interactions returns the client that answers interactions.
func (c *Client) Interactions() *Interactions { return c.interactions }

// Components returns the client for message components.
func (c *Client) Components() *Components { return &Components{rt: c.rt, def: c.def} }
