package dctl

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Herrscherd/dctl/internal/transport"
)

// Messages CRUDs channel messages.
type Messages struct {
	rt  transport.Doer
	def *defaults
}

// Send posts content to channelID (or the default channel when empty).
func (m *Messages) Send(ctx context.Context, channelID, content string) (*Message, error) {
	return m.post(ctx, channelID, map[string]any{fieldContent: content})
}

// SendFiles posts content with files attached to channelID (or the default
// channel). It returns [ErrTooManyFiles] when more than ten files are given.
func (m *Messages) SendFiles(ctx context.Context, channelID, content string, files []File) (*Message, error) {
	return m.postFiles(ctx, channelID, map[string]any{fieldContent: content}, files)
}

// Reply posts content as a reply to messageID in channelID (or the default channel).
func (m *Messages) Reply(ctx context.Context, channelID, messageID, content string) (*Message, error) {
	return m.post(ctx, channelID, replyBody(messageID, content))
}

// ReplyFiles posts content with files attached as a reply to messageID in
// channelID (or the default channel). It returns [ErrTooManyFiles] when more
// than ten files are given.
func (m *Messages) ReplyFiles(ctx context.Context, channelID, messageID, content string, files []File) (*Message, error) {
	return m.postFiles(ctx, channelID, replyBody(messageID, content), files)
}

func replyBody(messageID, content string) map[string]any {
	return map[string]any{
		fieldContent:        content,
		"message_reference": map[string]any{"message_id": messageID, "fail_if_not_exists": false},
	}
}

func (m *Messages) postFiles(ctx context.Context, channelID string, body map[string]any, files []File) (*Message, error) {
	if len(files) > maxAttachments {
		return nil, ErrTooManyFiles
	}
	for index, file := range files {
		if err := checkFile(file); err != nil {
			return nil, fmt.Errorf("file %d: %w", index, err)
		}
	}
	refs := make([]attachmentRef, len(files))
	parts := make([]transport.Part, len(files))
	for index, file := range files {
		refs[index] = attachmentRef{ID: index, Filename: file.Name, Description: file.Description}
		parts[index] = transport.Part{
			Name:        "files[" + strconv.Itoa(index) + "]",
			Filename:    file.Name,
			ContentType: file.ContentType,
			Content:     file.Content,
		}
	}
	body[fieldAttachments] = refs
	return m.post(ctx, channelID, transport.Multipart{Payload: body, Files: parts})
}

func checkFile(file File) error {
	switch {
	case file.Content == nil:
		return fmt.Errorf("%w: %q has no content", ErrInvalidFile, file.Name)
	case file.Name == "":
		return fmt.Errorf("%w: no name", ErrInvalidFile)
	case strings.ContainsFunc(file.Name, unicode.IsControl):
		return fmt.Errorf("%w: name %q holds a control character", ErrInvalidFile, file.Name)
	}
	return nil
}

type attachmentRef struct {
	ID          int    `json:"id"`
	Filename    string `json:"filename"`
	Description string `json:"description,omitempty"`
}

func (m *Messages) post(ctx context.Context, channelID string, body any) (*Message, error) {
	channel, err := m.def.resolveChannel(channelID)
	if err != nil {
		return nil, err
	}
	var msg Message
	if err := m.rt.Do(ctx, http.MethodPost, "/channels/"+seg(channel)+"/messages", body, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// Read returns up to limit (1..100, default 50) recent messages from channelID
// (or the default channel), oldest-first. When after is non-empty, only messages
// strictly newer than that id are returned (for polling).
func (m *Messages) Read(ctx context.Context, channelID string, limit int, after string) ([]Message, error) {
	channel, err := m.def.resolveChannel(channelID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	if after != "" {
		query.Set("after", after)
	}
	path := "/channels/" + seg(channel) + "/messages?" + query.Encode()
	var msgs []Message
	if err := m.rt.Do(ctx, http.MethodGet, path, nil, &msgs); err != nil {
		return nil, err
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// Get returns one message by id from channelID (or the default channel). Read
// only reaches the tail of a channel, so this is the way to a message named by
// something other than recency: the one a reply points at, for instance.
func (m *Messages) Get(ctx context.Context, channelID, messageID string) (*Message, error) {
	channel, err := m.def.resolveChannel(channelID)
	if err != nil {
		return nil, err
	}
	var msg Message
	if err := m.rt.Do(ctx, http.MethodGet, "/channels/"+seg(channel)+"/messages/"+seg(messageID), nil, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// Edit replaces the content of a message. An empty channelID falls back to the
// default channel and returns [ErrNoChannel] when none is set.
func (m *Messages) Edit(ctx context.Context, channelID, messageID, content string) (*Message, error) {
	channel, err := m.def.resolveChannel(channelID)
	if err != nil {
		return nil, err
	}
	var msg Message
	if err := m.rt.Do(ctx, http.MethodPatch, "/channels/"+seg(channel)+"/messages/"+seg(messageID),
		map[string]any{fieldContent: content}, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// Delete removes a message. An empty channelID falls back to the default channel
// and returns [ErrNoChannel] when none is set.
func (m *Messages) Delete(ctx context.Context, channelID, messageID string) error {
	channel, err := m.def.resolveChannel(channelID)
	if err != nil {
		return err
	}
	return m.rt.Do(ctx, http.MethodDelete, "/channels/"+seg(channel)+"/messages/"+seg(messageID), nil, nil)
}

// LastMessageAt returns the timestamp of the channel's most recent message, or
// the zero Time if the channel has no messages.
func (m *Messages) LastMessageAt(ctx context.Context, channelID string) (time.Time, error) {
	msgs, err := m.Read(ctx, channelID, 1, "")
	if err != nil {
		return time.Time{}, err
	}
	if len(msgs) == 0 {
		return time.Time{}, nil
	}
	sentAt, err := time.Parse(time.RFC3339, msgs[len(msgs)-1].Timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse message timestamp %q: %w", msgs[len(msgs)-1].Timestamp, err)
	}
	return sentAt, nil
}
