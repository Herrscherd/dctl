# dctl

**Discord REST API (v10) client for Go.** Dependency-free: standard library only,
enforced by a test that fails on any non-stdlib import.

No gateway websocket, no event loop, no CLI. Every call is an on-demand HTTP
request.

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
|---|---|
| `Messages()` | `Send` `Reply` `Read` `Get` `Edit` `Delete` `LastMessageAt` |
| `Channels()` | `List` `Get` `Type` `Create` `CreateUnder` `Rename` `Update` `Delete` `Ensure` `EnsureUnder` `Archive` |
| `Guilds()` | `List` `Sole` |
| `Members()` | `List` `Get` `Kick` `Ban` |
| `Roles()` | `List` `Create` `Update` `Delete` `Assign` `Unassign` |
| `Threads()` | `Start` `StartPrivate` `AddMember` `CreateForum` `ForumPost` |
| `Reactions()` | `Add` `Remove` |
| `Permissions()` | `Set` `Remove` |
| `Webhooks()` | `Create` `List` `Delete` `Execute` |
| `Components()` | `SendSelectMenu` `Ack` |
| `Interactions()` | `Register` `RegisterCommands` `List` `Create` `Edit` `Delete` `Registry` `Respond` `Defer` `RespondAutocomplete` `EditResponse` `UpsertStatusMessage` `AppID` |

## Defaults, and what `""` means

A channel-scoped operation accepts `""` for the configured default channel.

A guild-scoped one accepts `""` for the bot's sole guild, or for the guild pinned
by `WithGuild`, which a bot in several servers needs. `CreateUnder` and
`EnsureUnder` need neither: they read the guild off the parent category.

## Configuration

Nothing is read from the environment. Pass the token explicitly.

Without a token, `Enabled()` is false and every call returns `ErrDisabled`.

`WithHTTPClient` overrides the default 15 s client.

## Slash commands

`NewCommand(...).With(dctl.String(...), dctl.Sub(...))` builds a typed command.

`c.Interactions().Registry()` owns `Add`, `Sync` (which diffs against Discord and
creates, edits or deletes), plus `Dispatch` and `DispatchAutocomplete`.

Commands are registered on the default guild, which is instant but covers one
server. `WithGlobalCommands` registers them on the application instead, which
reaches every server the bot is in and takes up to an hour to propagate.

`Interaction.UserID()` is who invoked it, read from `member.user` in a guild and
from `user` in a DM. Read either field alone and permission checks see an empty
id in the other context.

## Escaping and secrets

Path segments are percent-escaped, and queries are built with `url.Values`.

`Webhook.Token` and `Interaction.Token` are `Secret`: they print as `[REDACTED]`
in logs and JSON, and `.Reveal()` reads the value.

## License

MIT
