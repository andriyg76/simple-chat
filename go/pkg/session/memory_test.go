package session

import (
	"context"
	"testing"

	"github.com/andriyg76/simple-chat/go/pkg/chatcore"
)

func TestMemoryMembersAndTitle(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	s, err := m.Create(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.UpdateTitle(ctx, "alice", s.ID, "Planning"); err != nil {
		t.Fatal(err)
	}
	if err := m.AddMember(ctx, "alice", s.ID, "bob"); err != nil {
		t.Fatal(err)
	}
	members, err := m.ListMembers(ctx, "alice", s.ID)
	if err != nil || len(members) != 2 {
		t.Fatalf("members %v err %v", members, err)
	}
	got, err := m.Get(ctx, "bob", s.ID)
	if err != nil || got.Title != "Planning" {
		t.Fatalf("bob view %+v err %v", got, err)
	}
	if err := m.RemoveMember(ctx, "alice", s.ID, "bob"); err != nil {
		t.Fatal(err)
	}
	if err := m.RemoveMember(ctx, "alice", s.ID, "alice"); err != chatcore.ErrLastMember {
		t.Fatalf("expected last member, got %v", err)
	}
}
