// Package dctl is a pure CLI client for the Discord bot REST API (v10).
// Auth is a bot token sent as `Authorization: Bot <token>`. No gateway/websocket:
// every call is on-demand HTTP. Mono-server by design (one bot token, one default
// channel).
package dctl

// Secret is a token-like string that redacts itself in logs (%v, %+v, %#v) and
// JSON so it can't leak by accident. Call Reveal to get the value when you must
// send it (e.g. Webhooks.Execute).
type Secret string

func (Secret) String() string               { return "[REDACTED]" }
func (Secret) GoString() string             { return "[REDACTED]" }
func (Secret) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (s Secret) Reveal() string             { return string(s) }

// Author identifies who wrote a message.
type Author struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Bot      bool   `json:"bot"`
}

// Attachment is a file uploaded alongside a message. URL points at the Discord CDN.
type Attachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
}

// EmbedMedia is an image carried by an embed. Only the URL is surfaced: it is
// what a reader needs to fetch the picture.
type EmbedMedia struct {
	URL string `json:"url"`
}

// EmbedField is one name/value pair in an embed's body.
type EmbedField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Embed is the rich block a bot or a link preview attaches to a message. Most of
// a webhook-driven channel's content lives here rather than in Content, so a
// reader that only looks at Content sees an empty message.
type Embed struct {
	Title       string       `json:"title"`
	Description string       `json:"description"`
	URL         string       `json:"url"`
	Image       *EmbedMedia  `json:"image"`
	Thumbnail   *EmbedMedia  `json:"thumbnail"`
	Fields      []EmbedField `json:"fields"`
}

// Message is the subset of a Discord message we surface.
type Message struct {
	ID          string       `json:"id"`
	ChannelID   string       `json:"channel_id"`
	Content     string       `json:"content"`
	Author      Author       `json:"author"`
	Timestamp   string       `json:"timestamp"`
	Attachments []Attachment `json:"attachments"`
	Embeds      []Embed      `json:"embeds"`
}

// Guild is a Discord server the bot belongs to.
type Guild struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Channel is a Discord channel. Type 0 is a text channel.
type Channel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     int    `json:"type"`
	GuildID  string `json:"guild_id,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
}

// Role is a Discord guild role.
type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       int    `json:"color"`
	Permissions string `json:"permissions"`
	Position    int    `json:"position"`
}

// GuildMember is a member of a guild.
type GuildMember struct {
	User  Author   `json:"user"`
	Nick  string   `json:"nick"`
	Roles []string `json:"roles"`
}
