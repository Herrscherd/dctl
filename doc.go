// Package dctl is a pure CLI client for the Discord bot REST API (v10).
// Auth is a bot token sent as `Authorization: Bot <token>`. No gateway/websocket:
// every call is on-demand HTTP. Mono-server by design (one bot token, one default
// channel).
package dctl
