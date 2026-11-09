package dctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Herrscherd/dctl/internal/transport"
)

type sentRequest struct {
	Method      string
	Path        string
	ContentType string
	Payload     map[string]any
	Files       map[string]sentFile
}

type sentFile struct {
	Filename    string
	ContentType string
	Body        string
}

func filesMessages(t *testing.T, handler http.HandlerFunc) *Messages {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Messages{rt: transport.NewHTTP("tok", transport.WithBase(srv.URL)), def: &defaults{channel: "def"}}
}

func captureRequest(tb testing.TB, r *http.Request) sentRequest {
	tb.Helper()
	got := sentRequest{
		Method:      r.Method,
		Path:        r.URL.Path,
		ContentType: r.Header.Get("Content-Type"),
		Files:       make(map[string]sentFile),
	}
	mr, err := r.MultipartReader()
	if err != nil {
		tb.Errorf("request is not multipart: %v", err)
		return got
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return got
		}
		if err != nil {
			tb.Errorf("reading part: %v", err)
			return got
		}
		body, err := io.ReadAll(part)
		if err != nil {
			tb.Errorf("reading part body: %v", err)
			return got
		}
		if part.FormName() == "payload_json" {
			if err := json.Unmarshal(body, &got.Payload); err != nil {
				tb.Errorf("payload_json is not JSON: %v", err)
			}
			continue
		}
		got.Files[part.FormName()] = sentFile{Filename: part.FileName(), ContentType: part.Header.Get("Content-Type"), Body: string(body)}
	}
}

func replyJSON(tb testing.TB, w http.ResponseWriter, status int, body string) {
	tb.Helper()
	w.WriteHeader(status)
	if _, err := io.WriteString(w, body); err != nil {
		tb.Errorf("writing response: %v", err)
	}
}

func requireSame(tb testing.TB, what string, got, want any) {
	tb.Helper()
	gotJSON, gotErr := json.Marshal(got)
	wantJSON, wantErr := json.Marshal(want)
	if gotErr != nil || wantErr != nil {
		tb.Fatalf("%s: marshal got %v, want %v", what, gotErr, wantErr)
	}
	if !bytes.Equal(gotJSON, wantJSON) {
		tb.Errorf("%s = %s, want %s", what, gotJSON, wantJSON)
	}
}

func TestMessagesFilesSendsPayloadAndFileParts(t *testing.T) {
	tests := []struct {
		name          string
		send          func(context.Context, *Messages, []File) (*Message, error)
		wantReference map[string]any
	}{
		{
			name: "send",
			send: func(ctx context.Context, m *Messages, files []File) (*Message, error) {
				return m.SendFiles(ctx, "", "voici", files)
			},
		},
		{
			name: "reply",
			send: func(ctx context.Context, m *Messages, files []File) (*Message, error) {
				return m.ReplyFiles(ctx, "", "m0", "voici", files)
			},
			wantReference: map[string]any{"message_id": "m0", "fail_if_not_exists": false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got sentRequest
			m := filesMessages(t, func(w http.ResponseWriter, r *http.Request) {
				got = captureRequest(t, r)
				replyJSON(t, w, http.StatusOK, `{"id":"m1","content":"voici"}`)
			})
			files := []File{
				{Name: "a.png", Content: strings.NewReader("PNGDATA"), ContentType: "image/png", Description: "un graphique"},
				{Name: "b.txt", Content: strings.NewReader("hello")},
			}
			msg, err := tt.send(context.Background(), m, files)
			if err != nil {
				t.Fatal(err)
			}
			if msg.ID != "m1" {
				t.Fatalf("msg = %+v", msg)
			}
			assertMultipartSend(t, got, tt.wantReference)
		})
	}
}

func assertMultipartSend(t *testing.T, got sentRequest, wantReference map[string]any) {
	t.Helper()
	if got.Method != http.MethodPost || got.Path != "/channels/def/messages" {
		t.Errorf("call = %s %s", got.Method, got.Path)
	}
	if !strings.HasPrefix(got.ContentType, "multipart/form-data; boundary=") {
		t.Errorf("content type = %q", got.ContentType)
	}
	wantPayload := map[string]any{
		"content": "voici",
		"attachments": []any{
			map[string]any{"id": float64(0), "filename": "a.png", "description": "un graphique"},
			map[string]any{"id": float64(1), "filename": "b.txt"},
		},
	}
	if wantReference != nil {
		wantPayload["message_reference"] = wantReference
	}
	requireSame(t, "payload_json", got.Payload, wantPayload)
	wantFiles := map[string]sentFile{
		"files[0]": {Filename: "a.png", ContentType: "image/png", Body: "PNGDATA"},
		"files[1]": {Filename: "b.txt", ContentType: "application/octet-stream", Body: "hello"},
	}
	requireSame(t, "file parts", got.Files, wantFiles)
}

func TestMessagesFilesLimitsAMessageToTenFiles(t *testing.T) {
	tests := []struct {
		name         string
		count        int
		wantErr      error
		wantRequests int
	}{
		{name: "ten files are sent", count: 10, wantRequests: 1},
		{name: "eleven files are refused", count: 11, wantErr: ErrTooManyFiles},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			m := filesMessages(t, func(w http.ResponseWriter, _ *http.Request) {
				requests++
				replyJSON(t, w, http.StatusOK, `{"id":"m1"}`)
			})
			files := make([]File, tt.count)
			for index := range files {
				files[index] = File{Name: "f.txt", Content: strings.NewReader("x")}
			}
			_, err := m.SendFiles(context.Background(), "", "", files)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if requests != tt.wantRequests {
				t.Errorf("requests = %d, want %d", requests, tt.wantRequests)
			}
		})
	}
}

func TestMessagesFilesRetriesARateLimitWithTheSameBytes(t *testing.T) {
	var sent []sentRequest
	m := filesMessages(t, func(w http.ResponseWriter, r *http.Request) {
		sent = append(sent, captureRequest(t, r))
		if len(sent) == 1 {
			replyJSON(t, w, http.StatusTooManyRequests, `{"retry_after":0.001}`)
			return
		}
		replyJSON(t, w, http.StatusOK, `{"id":"m1"}`)
	})
	files := []File{{Name: "a.txt", Content: strings.NewReader("hello")}}
	if _, err := m.SendFiles(context.Background(), "", "retry me", files); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 2 {
		t.Fatalf("requests = %d, want the rate limited request replayed once", len(sent))
	}
	requireSame(t, "retried request", sent[1], sent[0])
	if body := sent[1].Files["files[0]"].Body; body != "hello" {
		t.Errorf("retried file body = %q, want %q", body, "hello")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestMessagesFilesRefusesAnInvalidFileBeforeAnyRequest(t *testing.T) {
	tests := []struct {
		name    string
		file    File
		wantErr error
	}{
		{name: "no content", file: File{Name: "a.txt"}, wantErr: ErrInvalidFile},
		{name: "no name", file: File{Content: strings.NewReader("x")}, wantErr: ErrInvalidFile},
		{name: "newline in name", file: File{Name: "a\r\nX-Evil: 1.png", Content: strings.NewReader("x")}, wantErr: ErrInvalidFile},
		{name: "nul in name", file: File{Name: "a\x00.png", Content: strings.NewReader("x")}, wantErr: ErrInvalidFile},
		{name: "invalid utf-8 in name", file: File{Name: "a\xff.png", Content: strings.NewReader("x")}, wantErr: ErrInvalidFile},
		{name: "crlf in content type", file: File{Name: "a.txt", ContentType: "text/plain\r\nX-Injected: yes", Content: strings.NewReader("x")}, wantErr: ErrInvalidFile},
		{name: "unparsable content type", file: File{Name: "a.txt", ContentType: "text/", Content: strings.NewReader("x")}, wantErr: ErrInvalidFile},
		{name: "failing reader", file: File{Name: "a.txt", Content: failingReader{}}, wantErr: io.ErrUnexpectedEOF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			m := filesMessages(t, func(w http.ResponseWriter, _ *http.Request) {
				requests++
				replyJSON(t, w, http.StatusOK, `{"id":"m1"}`)
			})
			_, err := m.SendFiles(context.Background(), "", "x", []File{tt.file})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if requests != 0 {
				t.Errorf("requests = %d, want 0", requests)
			}
		})
	}
}

func TestMessagesFilesKeepsTheFilenameAsItIs(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{name: "accented", filename: "été.png"},
		{name: "quote and backslash", filename: `a"b\c.png`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got sentRequest
			m := filesMessages(t, func(w http.ResponseWriter, r *http.Request) {
				got = captureRequest(t, r)
				replyJSON(t, w, http.StatusOK, `{"id":"m1"}`)
			})
			if _, err := m.SendFiles(context.Background(), "", "", []File{{Name: tt.filename, Content: strings.NewReader("x")}}); err != nil {
				t.Fatal(err)
			}
			if name := got.Files["files[0]"].Filename; name != tt.filename {
				t.Errorf("part filename = %q, want %q", name, tt.filename)
			}
		})
	}
}

func TestMessagesFilesWithNoFileSendsAnEmptyAttachmentList(t *testing.T) {
	var got sentRequest
	m := filesMessages(t, func(w http.ResponseWriter, r *http.Request) {
		got = captureRequest(t, r)
		replyJSON(t, w, http.StatusOK, `{"id":"m1"}`)
	})
	if _, err := m.SendFiles(context.Background(), "", "hi", nil); err != nil {
		t.Fatal(err)
	}
	requireSame(t, "payload_json", got.Payload, map[string]any{"content": "hi", "attachments": []any{}})
	requireSame(t, "file parts", got.Files, make(map[string]sentFile))
}
