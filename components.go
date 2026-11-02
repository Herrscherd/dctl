package dctl

import (
	"context"
	"net/http"

	"github.com/Herrscherd/dctl/internal/transport"
)

// SelectOption is one entry in a select menu: Label is shown in the dropdown,
// Value is what the interaction submits when the entry is chosen.
type SelectOption struct {
	Label       string
	Value       string
	Description string
}

// Components sends message components (select menus) and acks their interactions.
type Components struct {
	rt  transport.Doer
	def *defaults
}

func choiceMenuComponents(customID string, options []SelectOption) []map[string]any {
	opts := make([]map[string]any, 0, len(options))
	for _, option := range options {
		fields := map[string]any{"label": clamp(option.Label), fieldValue: clamp(option.Value)}
		if option.Description != "" {
			fields["description"] = clamp(option.Description)
		}
		opts = append(opts, fields)
	}
	return []map[string]any{{
		fieldType: 1,
		fieldComponents: []map[string]any{{
			fieldType:   componentStringSelect,
			"custom_id": customID,
			"options":   opts,
		}},
	}}
}

// SendSelectMenu posts content with a single-select dropdown. When replyTo is
// set the message threads under it; customID routes the click back to the caller.
//
// It returns [ErrNoChannel] when channelID is empty and no default channel is set.
func (c *Components) SendSelectMenu(ctx context.Context, channelID, replyTo, content, customID string, options []SelectOption) (*Message, error) {
	channel, err := c.def.resolveChannel(channelID)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		fieldContent:    content,
		fieldComponents: choiceMenuComponents(customID, options),
	}
	if replyTo != "" {
		body["message_reference"] = map[string]any{"message_id": replyTo, "fail_if_not_exists": false}
	}
	var msg Message
	if err := c.rt.Do(ctx, http.MethodPost, "/channels/"+seg(channel)+"/messages", body, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// Ack acknowledges a component interaction with an UPDATE_MESSAGE (type 7):
// rewrites the message content and drops the menu so the click is confirmed
// and the dropdown can't be used twice. Must be sent within Discord's 3s deadline.
func (c *Components) Ack(ctx context.Context, interactionID, token, content string) error {
	body := map[string]any{
		fieldType: callbackUpdateMessage,
		fieldData: map[string]any{fieldContent: content, fieldComponents: []any{}},
	}
	return c.rt.Do(ctx, http.MethodPost, "/interactions/"+seg(interactionID)+"/"+seg(token)+"/callback", body, nil)
}

func clamp(text string) string {
	limit := maxOptionText
	if len(text) <= limit {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}
