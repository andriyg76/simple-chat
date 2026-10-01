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

func TestSubscribeReceivesIndependentMessagesAndReconnects(t *testing.T) {
	connections := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/v1/conversations/one/events" {
			t.Errorf("path = %s", r.URL.Path)
			return
		}
		connections++
		w.Header().Set("Content-Type", "text/event-stream")
		if connections == 1 {
			if r.Header.Get("Last-Event-ID") != "0" {
				t.Errorf("initial cursor = %q", r.Header.Get("Last-Event-ID"))
			}
			fmt.Fprint(w, "id: 1\nevent: message\ndata: {\"cursor\":1,\"messageId\":\"a\",\"sender\":\"client\",\"kind\":\"message\",\"text\":\"hello\"}\n\n")
		} else {
			if r.Header.Get("Last-Event-ID") != "1" {
				t.Errorf("reconnect cursor = %q", r.Header.Get("Last-Event-ID"))
			}
			fmt.Fprint(w, "id: 2\nevent: message\ndata: {\"cursor\":2,\"messageId\":\"b\",\"sender\":\"server\",\"kind\":\"message\",\"text\":\"reply\"}\n\n")
		}
	}))
	defer server.Close()
	client := Client{BaseURL: server.URL, Token: "token"}
	var events []Event
	err := client.Subscribe(context.Background(), "one", 0, func(event Event) error {
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

func TestSubscribeFallsBackAndKeepsLegacyReply(t *testing.T) {
	feedCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/events") {
			http.NotFound(w, r)
			return
		}
		feedCalls++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"cursor":1,"requestId":"req","messageId":"reply","kind":"reply","text":"old server"}`)
	}))
	defer server.Close()
	err := (Client{BaseURL: server.URL}).Subscribe(context.Background(), "one", 0, func(event Event) error {
		if event.Text != "old server" {
			t.Fatal(event)
		}
		return stop
	})
	if !errors.Is(err, stop) || feedCalls != 1 {
		t.Fatalf("fallback err=%v calls=%d", err, feedCalls)
	}
}

func TestSubscribeDoesNotFallbackOnAuthOrExpiredCursor(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"auth", http.StatusUnauthorized, `{"error":{"code":"unauthorized","message":"invalid token"}}`, nil},
		{"expired", http.StatusConflict, `{"error":{"code":"cursor_expired","message":"expired"}}`, ErrCursorExpired},
	} {
		t.Run(test.name, func(t *testing.T) {
			feedCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/feed") {
					feedCalls++
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			}))
			defer server.Close()
			err := (Client{BaseURL: server.URL}).Subscribe(context.Background(), "one", 0, func(Event) error { return nil })
			if err == nil || feedCalls != 0 {
				t.Fatalf("err=%v feedCalls=%d", err, feedCalls)
			}
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("err=%v, want %v", err, test.want)
			}
		})
	}
}

func TestMalformedSSEEventDoesNotFallback(t *testing.T) {
	feedCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/feed") {
			feedCalls++
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "id: 1\nevent: message\ndata: {invalid}\n\n")
	}))
	defer server.Close()
	err := (Client{BaseURL: server.URL}).Subscribe(context.Background(), "one", 0, func(Event) error { return nil })
	if err == nil || feedCalls != 0 {
		t.Fatalf("err=%v fallback calls=%d", err, feedCalls)
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
		fmt.Fprint(w, "id: 1\nevent: message\ndata: {\"cursor\":1,\"messageId\":\"m\",\"sender\":\"server\",\"kind\":\"message\",\"text\":\"after idle\"}\n\n")
	}))
	defer server.Close()
	client := Client{BaseURL: server.URL, HTTP: &http.Client{Timeout: 20 * time.Millisecond}}
	err := client.Subscribe(context.Background(), "one", 0, func(event Event) error {
		if event.Text != "after idle" {
			t.Fatal(event)
		}
		return stop
	})
	if !errors.Is(err, stop) || feedCalls != 0 {
		t.Fatalf("err=%v fallback calls=%d", err, feedCalls)
	}
	if client.HTTP.Timeout != 20*time.Millisecond {
		t.Fatalf("JSON client timeout changed: %v", client.HTTP.Timeout)
	}
}
