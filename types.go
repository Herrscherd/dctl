package dctl

import "io"

// Secret is a token-like string that redacts itself in logs (%v, %+v, %#v) and
// JSON so it can't leak by accident. Call Reveal to get the value when you must
// send it (e.g. Webhooks.Execute).
type Secret string

func (Secret) String() string { return "[REDACTED]" }

// GoString returns the redacted form, so %#v does not print the secret either.
func (Secret) GoString() string { return "[REDACTED]" }

// MarshalJSON encodes the secret as the redacted form, so it never reaches a
// JSON payload by accident.
func (Secret) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }

// Reveal returns the secret value. Call it only where the value must be sent.
func (s Secret) Reveal() string { return string(s) }

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

// File is a file sent with a message. Name is the filename Discord shows,
// Content the bytes (read once, when the message is sent), ContentType the media
// type of Content, and Description the optional alt text Discord keeps with it.
// Content must not be a nil pointer wrapped in the interface. An empty
// ContentType is sent as application/octet-stream; any other must parse as a
// media type. The whole message is
// held in memory before it is sent, so that a retry sends the same bytes.
type File struct {
	Name        string
	Content     io.Reader
	ContentType string
	Description string
}

// EmbedMedia is an image carried by an embed. URL is where the picture actually
// lives, which for a link preview is a third-party site; ProxyURL is Discord's
// own copy of it and is present whenever Discord could fetch it. A reader that
// only trusts Discord hosts wants ProxyURL, falling back to URL.
type EmbedMedia struct {
	URL      string `json:"url"`
	ProxyURL string `json:"proxy_url"`
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
