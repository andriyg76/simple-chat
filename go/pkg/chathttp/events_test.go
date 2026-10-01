package chathttp

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agushuley/simple-chat/go/pkg/chatcore"
)

type eventStore struct{}

func (eventStore) List(context.Context, string, bool) ([]chatcore.Session, error) { return nil, nil }
func (eventStore) Create(context.Context, string) (chatcore.Session, error) {
	return chatcore.Session{}, nil
}
func (eventStore) Get(_ context.Context, participant, id string) (chatcore.Session, error) {
	if participant != "owner" || id != "one" {
		return chatcore.Session{}, chatcore.ErrNotFound
	}
	return chatcore.Session{ID: id}, nil
}
func (eventStore) Touch(context.Context, string, string) error                { return nil }
func (eventStore) Save(context.Context, string, string, string, string) error { return nil }

func TestSSEReplayHeartbeatAndDisconnect(t *testing.T) {
	h := &Handler{core: chatcore.New(eventStore{})}
	h.core.Emit("one", chatcore.Event{Kind: "message", Sender: "server", Text: "hello"})
	previous := heartbeatInterval
	heartbeatInterval = 10 * time.Millisecond
	defer func() { heartbeatInterval = previous }()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("code", "one")
		h.streamEvents(w, r, "owner")
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	req.Header.Set("Last-Event-ID", "0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal(resp.Header)
	}
	reader := bufio.NewReader(resp.Body)
	var lines []string
	for len(lines) < 5 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, line)
	}
	joined := strings.Join(lines, "")
	if !strings.Contains(joined, "id: 1\n") || !strings.Contains(joined, "event: message\n") || !strings.Contains(joined, "\"text\":\"hello\"") || !strings.Contains(joined, ": heartbeat\n") {
		t.Fatalf("SSE = %q", joined)
	}
	cancel()
	resp.Body.Close()
}

func TestSSERejectsExpiredCursorBeforeHeaders(t *testing.T) {
	h := &Handler{core: chatcore.New(eventStore{})}
	for range 101 {
		h.core.Emit("one", chatcore.Event{Kind: "message", Text: "x"})
	}
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	req.SetPathValue("code", "one")
	req.Header.Set("Last-Event-ID", "0")
	w := httptest.NewRecorder()
	h.streamEvents(w, req, "owner")
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "cursor_expired") {
		t.Fatalf("response = %d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/events", nil)
	req.SetPathValue("code", "one")
	req.Header.Set("Last-Event-ID", "1")
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	h.streamEvents(w, req, "owner")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "id: 2\n") {
		t.Fatalf("boundary = %d %s", w.Code, w.Body.String())
	}
}

func TestSSERequiresBearerBeforeOpeningStream(t *testing.T) {
	h := &Handler{core: chatcore.New(eventStore{})}
	req := httptest.NewRequest(http.MethodGet, "/chat/v1/conversations/one/events", nil)
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "unauthorized") {
		t.Fatalf("response = %d %s", w.Code, w.Body.String())
	}
}
