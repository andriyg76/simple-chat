package chatcore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu       sync.Mutex
	sessions map[string]Session
}

func (s *memoryStore) List(_ context.Context, p string, _ bool) ([]Session, error) {
	if p != "owner" {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Session{}
	for _, v := range s.sessions {
		out = append(out, v)
	}
	return out, nil
}
func (s *memoryStore) Create(_ context.Context, p string) (Session, error) {
	if p != "owner" {
		return Session{}, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v := Session{ID: fmt.Sprint(len(s.sessions) + 1)}
	s.sessions[v.ID] = v
	return v, nil
}
func (s *memoryStore) Get(_ context.Context, p, id string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.sessions[id]
	if p != "owner" || !ok {
		return Session{}, ErrNotFound
	}
	return v, nil
}
func TestGenericSessionsAndDelivery(t *testing.T) {
	ctx := context.Background()
	store := &memoryStore{sessions: map[string]Session{}}
	core := New(store)
	session, err := core.Create(ctx, "owner")
	if err != nil {
		t.Fatal(err)
	}
	list, err := core.List(ctx, "owner", false)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v, %v", list, err)
	}
	if _, err := core.Publish(ctx, "other", session.ID, "client", "denied", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unauthorized publish = %v", err)
	}
	first, err := core.Publish(ctx, "owner", session.ID, "client", "hello", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := core.Publish(ctx, "owner", session.ID, "server", "hi", "")
	if err != nil {
		t.Fatal(err)
	}
	if first.Cursor != 1 || second.Cursor != 2 || first.MessageID == second.MessageID {
		t.Fatalf("events = %+v %+v", first, second)
	}
	got, found, err := core.Next(ctx, "owner", session.ID, 1, 0)
	if err != nil || !found || got.Text != "hi" {
		t.Fatalf("next = %+v %v %v", got, found, err)
	}
}

func TestBroadcastReplayBoundaryAndCancellation(t *testing.T) {
	ctx := context.Background()
	core := New(&memoryStore{sessions: map[string]Session{"one": {ID: "one"}}})
	results := make(chan Event, 2)
	for range 2 {
		go func() { event, _, _ := core.Next(ctx, "owner", "one", 0, time.Second); results <- event }()
	}
	time.Sleep(10 * time.Millisecond)
	core.Emit("one", Event{Kind: "message", Text: "wake"})
	for range 2 {
		select {
		case event := <-results:
			if event.Text != "wake" {
				t.Fatal(event)
			}
		case <-time.After(time.Second):
			t.Fatal("subscriber did not wake")
		}
	}
	for i := 0; i < 100; i++ {
		core.Emit("one", Event{Kind: "message", Text: "more"})
	}
	if _, _, err := core.Next(ctx, "owner", "one", 0, 0); !errors.Is(err, ErrCursorExpired) {
		t.Fatalf("expired = %v", err)
	}
	if event, found, err := core.Next(ctx, "owner", "one", 1, 0); err != nil || !found || event.Cursor != 2 {
		t.Fatalf("boundary = %+v %v %v", event, found, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := core.Next(canceled, "owner", "one", 101, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel = %v", err)
	}
}
