package dctl

import (
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
	guild      string
	global     bool
}

// WithHTTPClient overrides the default 15s-timeout HTTP client.
func WithHTTPClient(h *http.Client) ClientOption {
	return func(c *clientConfig) { c.httpClient = h }
}

// WithGuild pins the guild that guild-scoped ops target when no explicit id is
// passed. Without it they fall back to Guilds().Sole, which errors when the bot
// is in more than one server — a bot invited to a second guild would otherwise
// lose command registration entirely.
func WithGuild(id string) ClientOption {
	return func(c *clientConfig) { c.guild = id }
}

// WithGlobalCommands registers slash commands on the application rather than on
// one guild, so every server the bot is in gets them — the only scope that
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
	rt := transport.NewHTTP(token, topts...)
	c := newWith(rt, defaultChannel)
	c.def.guild = cfg.guild
	c.def.globalCommands = cfg.global
	return c
}

// newWith wires a Client around an arbitrary Doer (used by tests with a stub).
func newWith(rt transport.Doer, defaultChannel string) *Client {
	guilds := &Guilds{rt: rt}
	def := &defaults{rt: rt, channel: defaultChannel, guilds: guilds}
	return &Client{
		rt:           rt,
		def:          def,
		guilds:       guilds,
		interactions: &Interactions{rt: rt, def: def},
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

// Sub-client accessors. Each shares the transport and (where relevant) the
// default channel/guild resolver.
func (c *Client) Guilds() *Guilds             { return c.guilds }
func (c *Client) Messages() *Messages         { return &Messages{rt: c.rt, def: c.def} }
func (c *Client) Channels() *Channels         { return &Channels{rt: c.rt, def: c.def} }
func (c *Client) Roles() *Roles               { return &Roles{rt: c.rt, def: c.def} }
func (c *Client) Members() *Members           { return &Members{rt: c.rt, def: c.def} }
func (c *Client) Reactions() *Reactions       { return &Reactions{rt: c.rt, def: c.def} }
func (c *Client) Threads() *Threads           { return &Threads{rt: c.rt, def: c.def} }
func (c *Client) Permissions() *Permissions   { return &Permissions{rt: c.rt} }
func (c *Client) Webhooks() *Webhooks         { return &Webhooks{rt: c.rt} }
func (c *Client) Interactions() *Interactions { return c.interactions }
func (c *Client) Components() *Components     { return &Components{rt: c.rt, def: c.def} }
