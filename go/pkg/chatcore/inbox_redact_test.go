package chatcore

import (
	"context"
	"testing"
)

func TestRedactedConversationIsSkippedButAdvancesCursor(t *testing.T) {
	c := &Core{inboxFeeds: map[string]*inboxJournal{}}
	c.appendInbox("guest", InboxEvent{SessionID: "shared", Kind: "message", Text: "secret 1"})
	c.appendInbox("guest", InboxEvent{SessionID: "own", Kind: "message", Text: "mine"})
	c.appendInbox("guest", InboxEvent{SessionID: "shared", Kind: "message", Text: "secret 2"})

	c.redactInboxConversation("guest", "shared")

	events, cursor, err := c.InboxNext(context.Background(), "guest", 0, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Text != "mine" || cursor != 3 {
		t.Fatalf("events = %+v cursor = %d", events, cursor)
	}
}
