# dctl

**Discord REST API (v10) client for Go.** Dependency-free — standard library
only, enforced by a test that fails on any non-stdlib import. No gateway
websocket, no event loop, no CLI: every call is an on-demand HTTP request.

```sh
go get github.com/Herrscherd/dctl
```

```go
c := dctl.New(token, defaultChannelID)

msg, _ := c.Messages().Send(ctx, "", "deploy finished") // "" => default channel
msgs, _ := c.Messages().Read(ctx, "", 20, "")           // oldest-first
```

## Resources

| Accessor | Operations |
|----------|------------|
| `Messages()` | `Send` `Reply` `Read` `Edit` `Delete` `LastMessageAt` |
| `Channels()` | `List` `Get` `Type` `Create` `CreateUnder` `Rename` `Update` `Delete` `Ensure` `EnsureUnder` `Archive` |
| `Guilds()` | `List` `Sole` |
| `Members()` | `List` `Get` `Kick` `Ban` |
| `Roles()` | `List` `Create` `Update` `Delete` `Assign` `Unassign` |
| `Threads()` | `Start` `CreateForum` `ForumPost` |
| `Reactions()` | `Add` `Remove` |
| `Permissions()` | `Set` `Remove` |
| `Webhooks()` | `Create` `List` `Delete` `Execute` |
| `Components()` | `SendSelectMenu` `Ack` |
| `Interactions()` | `Register` `RegisterCommands` `List` `Create` `Edit` `Delete` `Registry` `Respond` `Defer` `RespondAutocomplete` `EditResponse` `UpsertStatusMessage` `AppID` |

## Notes

Channel-scoped ops accept `""` for the configured default channel; guild-scoped
ops accept `""` for the bot's sole guild — or for the guild pinned by
`WithGuild`, which a bot in several servers needs. Nothing is read from the
environment — pass the token explicitly. Without one, `Enabled()` is false and
every call returns `ErrDisabled`. `WithHTTPClient` overrides the default 15s
client.

Slash commands: `NewCommand(...).With(dctl.String(...), dctl.Sub(...))` builds a
typed command; `c.Interactions().Registry()` owns `Add`, `Sync` (diff against
Discord: create/edit/delete), and `Dispatch` / `DispatchAutocomplete`. They are
registered on the default guild — instant, one server — or on the application
with `WithGlobalCommands`, which reaches every server the bot is in and takes up
to an hour to propagate.

Path segments are percent-escaped and queries built with `url.Values`.
`Webhook.Token` and `Interaction.Token` are `Secret` — `[REDACTED]` in logs and
JSON, `.Reveal()` to read.

## License

MIT
