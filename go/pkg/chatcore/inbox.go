package chatcore

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InboxEvent is the profile-scoped delivery unit (inbox SSE / feed).
type InboxEvent struct {
	InboxCursor int64     `json:"inboxCursor"`
	Cursor      int64     `json:"cursor"`
	SessionID   string    `json:"sessionId,omitempty"`
	RequestID   string    `json:"requestId,omitempty"`
	MessageID   string    `json:"messageId"`
	Sender      string    `json:"sender,omitempty"`
	Kind        string    `json:"kind"`
	Text        string    `json:"text"`
	CreatedAt   time.Time `json:"createdAt,omitempty"`
}

type inboxJournal struct {
	mu     sync.Mutex
	next   int64
	events []InboxEvent
	wake   chan struct{}
}

func (c *Core) inboxJournal(profileID string) *inboxJournal {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inboxFeeds[profileID] == nil {
		c.inboxFeeds[profileID] = &inboxJournal{wake: make(chan struct{})}
	}
	return c.inboxFeeds[profileID]
}

// InboxCursor returns the current inbox cursor for a profile.
func (c *Core) InboxCursor(_ context.Context, participant string) (int64, error) {
	j := c.inboxJournal(participant)
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.next, nil
}

func (c *Core) fanOutToInbox(ctx context.Context, actor, conversationCode string, event Event) {
	recipients, err := c.deliveryRecipients(ctx, actor, conversationCode)
	if err != nil || len(recipients) == 0 {
		recipients = []string{actor}
	}
	payload := eventToInbox(event)
	for _, profileID := range recipients {
		c.appendInbox(profileID, payload)
	}
}

func (c *Core) deliveryRecipients(ctx context.Context, actor, conversationCode string) ([]string, error) {
	if c.members != nil {
		list, err := c.members.ListMembers(ctx, actor, conversationCode)
		if err == nil && len(list) > 0 {
			return list, nil
		}
	}
	return []string{actor}, nil
}

func eventToInbox(event Event) InboxEvent {
	return InboxEvent{
		Cursor:    event.Cursor,
		SessionID: event.SessionID,
		RequestID: event.RequestID,
		MessageID: event.MessageID,
		Sender:    event.Sender,
		Kind:      event.Kind,
		Text:      event.Text,
		CreatedAt: event.CreatedAt,
	}
}

func (c *Core) appendInbox(profileID string, template InboxEvent) InboxEvent {
	j := c.inboxJournal(profileID)
	j.mu.Lock()
	defer j.mu.Unlock()
	j.next++
	template.InboxCursor = j.next
	if template.MessageID == "" {
		template.MessageID = uuid.NewString()
	}
	// Metadata events have no author; the protocol requires a sender, so they come from the server.
	if template.Sender == "" {
		template.Sender = "server"
	}
	if template.CreatedAt.IsZero() {
		template.CreatedAt = time.Now().UTC()
	}
	j.events = append(j.events, template)
	if len(j.events) > 100 {
		j.events = j.events[len(j.events)-100:]
	}
	close(j.wake)
	j.wake = make(chan struct{})
	return template
}

// EmitInbox delivers a non-message event to one profile inbox.
func (c *Core) EmitInbox(profileID, sessionID, kind, text string) InboxEvent {
	return c.appendInbox(profileID, InboxEvent{
		SessionID: sessionID,
		Kind:      kind,
		Text:      text,
	})
}

// EmitInboxToConversationMembers fan-out a metadata-style inbox event to all members.
func (c *Core) EmitInboxToConversationMembers(ctx context.Context, actor, conversationCode, kind, text string) {
	recipients, err := c.deliveryRecipients(ctx, actor, conversationCode)
	if err != nil || len(recipients) == 0 {
		recipients = []string{actor}
	}
	template := InboxEvent{SessionID: conversationCode, Kind: kind, Text: text}
	for _, profileID := range recipients {
		c.appendInbox(profileID, template)
	}
}

// InboxNext returns up to limit inbox events strictly after inboxCursor.
func (c *Core) InboxNext(ctx context.Context, participant string, inboxCursor int64, limit int, timeout time.Duration) ([]InboxEvent, int64, error) {
	if inboxCursor < 0 {
		return nil, 0, ErrInvalidCursor
	}
	if limit <= 0 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}
	j := c.inboxJournal(participant)
	for {
		j.mu.Lock()
		if len(j.events) != 0 && inboxCursor < j.events[0].InboxCursor-1 {
			j.mu.Unlock()
			return nil, 0, ErrInboxCursorExpired
		}
		if inboxCursor > j.next {
			j.mu.Unlock()
			return nil, 0, ErrInvalidCursor
		}
		var batch []InboxEvent
		advanced := inboxCursor
		for _, event := range j.events {
			if event.InboxCursor <= inboxCursor {
				continue
			}
			advanced = event.InboxCursor
			// Redacted entries keep cursor positions stable but are never delivered.
			if event.Kind != "" {
				batch = append(batch, event)
				if len(batch) >= limit {
					break
				}
			}
		}
		wake := j.wake
		j.mu.Unlock()
		if advanced > inboxCursor {
			return batch, advanced, nil
		}
		if timeout <= 0 {
			return nil, inboxCursor, nil
		}
		timer := time.NewTimer(timeout)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, 0, ctx.Err()
		case <-timer.C:
			return nil, inboxCursor, nil
		case <-wake:
			timer.Stop()
		}
	}
}

// redactInboxConversation turns a former member's buffered events for a conversation into cursor-only
// placeholders, so they are never delivered while existing cursors stay valid.
func (c *Core) redactInboxConversation(profileID, conversationCode string) {
	j := c.inboxJournal(profileID)
	j.mu.Lock()
	defer j.mu.Unlock()
	for i, event := range j.events {
		if event.SessionID == conversationCode {
			j.events[i] = InboxEvent{InboxCursor: event.InboxCursor}
		}
	}
}

// TitleUpdatedJSON builds conversation_updated payload text.
func TitleUpdatedJSON(title string) string {
	raw, _ := json.Marshal(map[string]string{"title": title})
	return string(raw)
}

// MemberPayloadJSON builds member_added / member_removed payload text.
func MemberPayloadJSON(profileID string) string {
	raw, _ := json.Marshal(map[string]string{"profileId": profileID})
	return string(raw)
}
