package chatcore_test

import (
	"context"
	"testing"

	"github.com/andriyg76/simple-chat/go/pkg/chatcore"
	"github.com/andriyg76/simple-chat/go/pkg/session"
)

func TestMemberLeavingNotifiesRemainingMembers(t *testing.T) {
	ctx := context.Background()
	store := session.NewMemory()
	c := chatcore.New(store).WithMembers(store)
	s, err := c.Create(ctx, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddMember(ctx, "owner", s.ID, "guest"); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveMember(ctx, "guest", s.ID, "guest"); err != nil {
		t.Fatalf("leaving own conversation: %v", err)
	}
	events, _, err := c.InboxNext(ctx, "owner", 0, 50, 0)
	if err != nil || len(events) == 0 || events[len(events)-1].Kind != "member_removed" || events[len(events)-1].Sender != "server" {
		t.Fatalf("owner events = %+v err = %v", events, err)
	}
}
