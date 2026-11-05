package dctl

import (
	"encoding/json"
	"testing"
)

func TestInteractionDecodesLocales(t *testing.T) {
	const payload = `{
		"id": "1100000000000000001",
		"application_id": "1100000000000000002",
		"type": 2,
		"token": "interaction-token",
		"version": 1,
		"guild_id": "1100000000000000003",
		"guild_locale": "en-US",
		"channel_id": "1100000000000000004",
		"locale": "fr",
		"member": {
			"user": {"id": "1100000000000000005", "username": "shan", "bot": false}
		},
		"data": {
			"id": "1100000000000000006",
			"name": "ping",
			"type": 1
		}
	}`

	var in Interaction
	if err := json.Unmarshal([]byte(payload), &in); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if in.Locale != "fr" {
		t.Errorf("Locale = %q, want %q", in.Locale, "fr")
	}
	if in.GuildLocale != "en-US" {
		t.Errorf("GuildLocale = %q, want %q", in.GuildLocale, "en-US")
	}
}
