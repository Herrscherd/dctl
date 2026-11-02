// Package dctl is a pure CLI client for the Discord bot REST API (v10).
// Auth is a bot token sent as `Authorization: Bot <token>`. No gateway/websocket:
// every call is on-demand HTTP. Mono-server by design (one bot token, one default
// channel).
//
// A call that acts on a server without an explicit guild id uses the server
// pinned with [WithGuild], or else the bot's only server. It returns [ErrNoGuild]
// when the bot is in no server and an [*AmbiguousGuildError] when it is in
// several. A call that Discord rejects returns an error.
package dctl
