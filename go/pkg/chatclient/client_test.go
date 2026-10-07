package chatclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var stop = errors.New("received enough")

func TestSubscribeInboxReceivesMessagesAndReconnects(t *testing.T) {
	connections := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/v1/inbox/events" {
			t.Errorf("path = %s", r.URL.Path)
			return
		}
		connections++
		w.Header().Set("Content-Type", "text/event-stream")
		if connections == 1 {
			fmt.Fprint(w, "id: 1\nevent: message\ndata: {\"inboxCursor\":1,\"cursor\":1,\"sessionId\":\"one\",\"messageId\":\"a\",\"sender\":\"client\",\"kind\":\"message\",\"text\":\"hello\"}\n\n")
		} else {
			fmt.Fprint(w, "id: 2\nevent: message\ndata: {\"inboxCursor\":2,\"cursor\":2,\"sessionId\":\"one\",\"messageId\":\"b\",\"sender\":\"server\",\"kind\":\"message\",\"text\":\"reply\"}\n\n")
		}
	}))
	defer server.Close()
	client := Client{BaseURL: server.URL, Token: "token"}
	var events []InboxEvent
	err := client.SubscribeInbox(context.Background(), 0, func(event InboxEvent) error {
		events = append(events, event)
		if len(events) == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) || connections != 2 || events[0].Sender != "client" || events[1].Sender != "server" {
		t.Fatalf("events=%+v connections=%d err=%v", events, connections, err)
	}
}

func TestSubscribeInboxFallsBackToFeed(t *testing.T) {
	feedCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/events") {
			http.NotFound(w, r)
			return
		}
		feedCalls++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"inboxCursor":1,"events":[{"inboxCursor":1,"cursor":1,"sessionId":"one","messageId":"m","kind":"message","text":"from feed"}]}`)
	}))
	defer server.Close()
	err := (Client{BaseURL: server.URL}).SubscribeInbox(context.Background(), 0, func(event InboxEvent) error {
		if event.Text != "from feed" {
			t.Fatal(event)
		}
		return stop
	})
	if !errors.Is(err, stop) || feedCalls < 1 {
		t.Fatalf("fallback err=%v calls=%d", err, feedCalls)
	}
}

func TestSSEStaysOpenPastJSONClientTimeout(t *testing.T) {
	feedCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/feed") {
			feedCalls++
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		time.Sleep(80 * time.Millisecond)
		fmt.Fprint(w, "id: 1\nevent: message\ndata: {\"inboxCursor\":1,\"cursor\":1,\"sessionId\":\"one\",\"messageId\":\"m\",\"sender\":\"server\",\"kind\":\"message\",\"text\":\"after idle\"}\n\n")
	}))
	defer server.Close()
	client := Client{BaseURL: server.URL, HTTP: &http.Client{Timeout: 20 * time.Millisecond}}
	err := client.SubscribeInbox(context.Background(), 0, func(event InboxEvent) error {
		if event.Text != "after idle" {
			t.Fatal(event)
		}
		return stop
	})
	if !errors.Is(err, stop) || feedCalls != 0 {
		t.Fatalf("err=%v fallback calls=%d", err, feedCalls)
	}
}
