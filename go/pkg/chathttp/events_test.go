package chathttp

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andriyg76/simple-chat/go/pkg/chatcore"
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

func testHandler() *Handler {
	return New(eventStore{}, Config{BearerToken: "secret", ParticipantID: "owner"})
}

func TestInboxSSEReplayHeartbeatAndDisconnect(t *testing.T) {
	h := testHandler()
	ctx := context.Background()
	if _, err := h.core.Publish(ctx, "owner", "one", "server", "hello", ""); err != nil {
		t.Fatal(err)
	}
	previous := heartbeatInterval
	heartbeatInterval = 10 * time.Millisecond
	defer func() { heartbeatInterval = previous }()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.streamInbox(w, r, "owner")
	}))
	defer server.Close()
	reqCtx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, server.URL, nil)
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

func TestInboxSSERejectsExpiredCursorBeforeHeaders(t *testing.T) {
	h := testHandler()
	ctx := context.Background()
	for range 101 {
		if _, err := h.core.Publish(ctx, "owner", "one", "server", "x", ""); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/inbox/events", nil)
	req.Header.Set("Last-Event-ID", "0")
	w := httptest.NewRecorder()
	h.streamInbox(w, req, "owner")
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "inbox_cursor_expired") {
		t.Fatalf("response = %d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/inbox/events", nil)
	req.Header.Set("Last-Event-ID", "1")
	reqCtx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(reqCtx)
	w = httptest.NewRecorder()
	h.streamInbox(w, req, "owner")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "id: 2\n") {
		t.Fatalf("boundary = %d %s", w.Code, w.Body.String())
	}
}

func TestInboxSSERequiresBearerBeforeOpeningStream(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/chat/v1/inbox/events", nil)
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "unauthorized") {
		t.Fatalf("response = %d %s", w.Code, w.Body.String())
	}
}
