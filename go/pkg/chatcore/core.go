// Package chatcore provides session operations and bounded, ordered chat delivery.
// Storage and identity are supplied by adapters; no agent is required.
package chatcore

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound        = errors.New("chat session not found")
	ErrMessageRequired = errors.New("message is required")
	ErrCursorExpired   = errors.New("cursor_expired")
	ErrInvalidCursor        = errors.New("invalid cursor")
	ErrInboxCursorExpired   = errors.New("inbox_cursor_expired")
)

// Session is a chat conversation, not a bearer authentication session.
type Session struct {
	ID            string    `json:"code"`
	Title         string    `json:"title"`
	LastMessageAt time.Time `json:"lastMessageAt"`
	ReviewSummary string    `json:"reviewSummary,omitempty"`
}

// Store applies participant access checks to every operation.
// Persistence (messages, profiles) belongs in the product layer, not chatcore.
type Store interface {
	List(context.Context, string, bool) ([]Session, error)
	Create(context.Context, string) (Session, error)
	Get(context.Context, string, string) (Session, error)
}

type Event struct {
	Cursor    int64     `json:"cursor"`
	SessionID string    `json:"sessionId,omitempty"`
	RequestID string    `json:"requestId,omitempty"`
	MessageID string    `json:"messageId"`
	Sender    string    `json:"sender,omitempty"`
	Kind      string    `json:"kind"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

type Core struct {
	store    Store
	members  MembersStore
	metadata MetadataStore
	mu         sync.Mutex
	feeds      map[string]*journal
	inboxFeeds map[string]*inboxJournal
}

type journal struct {
	mu     sync.Mutex
	next   int64
	events []Event
	wake   chan struct{}
}

func New(store Store) *Core {
	return &Core{store: store, feeds: make(map[string]*journal), inboxFeeds: make(map[string]*inboxJournal)}
}

// WithMembers attaches optional multi-user membership storage (same product adapter may implement Store and MembersStore).
func (c *Core) WithMembers(members MembersStore) *Core {
	c.members = members
	return c
}

func (c *Core) WithMetadata(metadata MetadataStore) *Core {
	c.metadata = metadata
	return c
}

func (c *Core) List(ctx context.Context, participant string, archived bool) ([]Session, error) {
	return c.store.List(ctx, participant, archived)
}

func (c *Core) Create(ctx context.Context, participant string) (Session, error) {
	return c.store.Create(ctx, participant)
}

func (c *Core) Session(ctx context.Context, participant, id string) (Session, error) {
	return c.store.Get(ctx, participant, id)
}

func (c *Core) journal(id string) *journal {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.feeds[id] == nil {
		c.feeds[id] = &journal{wake: make(chan struct{})}
	}
	return c.feeds[id]
}

// Cursor returns the current delivery cursor after checking access.
func (c *Core) Cursor(ctx context.Context, participant, id string) (int64, error) {
	if _, err := c.Session(ctx, participant, id); err != nil {
		return 0, err
	}
	j := c.journal(id)
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.next, nil
}

// Publish posts a message from any participant or server integration.
func (c *Core) Publish(ctx context.Context, participant, id, sender, text, requestID string) (Event, error) {
	if strings.TrimSpace(text) == "" {
		return Event{}, ErrMessageRequired
	}
	if _, err := c.Session(ctx, participant, id); err != nil {
		return Event{}, err
	}
	event := c.Emit(id, Event{MessageID: uuid.NewString(), RequestID: requestID, Sender: sender, Kind: "message", Text: text})
	c.fanOutToInbox(ctx, participant, id, event)
	return event, nil
}

// Emit publishes an integration event after the integration has checked access.
// Use Publish for chat messages.
func (c *Core) Emit(id string, event Event) Event {
	j := c.journal(id)
	j.mu.Lock()
	defer j.mu.Unlock()
	j.next++
	event.Cursor = j.next
	event.SessionID = id
	if event.MessageID == "" {
		event.MessageID = uuid.NewString()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	j.events = append(j.events, event)
	if len(j.events) > 100 {
		j.events = j.events[len(j.events)-100:]
	}
	close(j.wake)
	j.wake = make(chan struct{})
	return event
}

// Next returns the first event strictly after cursor. Its second result is
// false on timeout. Cursor 0 is valid until event 1 has been evicted.
func (c *Core) Next(ctx context.Context, participant, id string, cursor int64, timeout time.Duration) (Event, bool, error) {
	if _, err := c.Session(ctx, participant, id); err != nil {
		return Event{}, false, err
	}
	if cursor < 0 {
		return Event{}, false, ErrInvalidCursor
	}
	j := c.journal(id)
	for {
		j.mu.Lock()
		if len(j.events) != 0 && cursor < j.events[0].Cursor-1 {
			j.mu.Unlock()
			return Event{}, false, ErrCursorExpired
		}
		if cursor > j.next {
			j.mu.Unlock()
			return Event{}, false, ErrInvalidCursor
		}
		for _, event := range j.events {
			if event.Cursor > cursor {
				j.mu.Unlock()
				// Membership may have changed while this reader waited.
				if _, err := c.Session(ctx, participant, id); err != nil {
					return Event{}, false, err
				}
				return event, true, nil
			}
		}
		current, wake := j.next, j.wake
		j.mu.Unlock()
		if timeout <= 0 {
			return Event{Cursor: current}, false, nil
		}
		timer := time.NewTimer(timeout)
		select {
		case <-ctx.Done():
			timer.Stop()
			return Event{}, false, ctx.Err()
		case <-timer.C:
			return Event{Cursor: current}, false, nil
		case <-wake:
			timer.Stop()
		}
	}
}

// wakeConversation releases readers waiting in Next so they re-check membership.
func (c *Core) wakeConversation(id string) {
	j := c.journal(id)
	j.mu.Lock()
	defer j.mu.Unlock()
	close(j.wake)
	j.wake = make(chan struct{})
}
