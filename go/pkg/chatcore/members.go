package chatcore

import (
	"context"
	"errors"
)

var (
	ErrMemberNotFound    = errors.New("member_not_found")
	ErrMemberExists      = errors.New("member_already_present")
	ErrLastMember        = errors.New("last_member")
	ErrMembersNotSupported = errors.New("members_not_supported")
)

// MembersStore is implemented by the product layer (persistence, ACL). Core checks conversation access via Store.Get first.
type MembersStore interface {
	ListMembers(ctx context.Context, actorParticipant, conversationCode string) ([]string, error)
	AddMember(ctx context.Context, actorParticipant, conversationCode, profileID string) error
	RemoveMember(ctx context.Context, actorParticipant, conversationCode, profileID string) error
}

func (c *Core) ListMembers(ctx context.Context, actor, conversationCode string) ([]string, error) {
	if c.members == nil {
		return nil, ErrMembersNotSupported
	}
	if _, err := c.Session(ctx, actor, conversationCode); err != nil {
		return nil, err
	}
	return c.members.ListMembers(ctx, actor, conversationCode)
}

func (c *Core) AddMember(ctx context.Context, actor, conversationCode, profileID string) error {
	if c.members == nil {
		return ErrMembersNotSupported
	}
	if _, err := c.Session(ctx, actor, conversationCode); err != nil {
		return err
	}
	if err := c.members.AddMember(ctx, actor, conversationCode, profileID); err != nil {
		return err
	}
	c.EmitInboxToConversationMembers(ctx, actor, conversationCode, "member_added", MemberPayloadJSON(profileID))
	return nil
}

func (c *Core) RemoveMember(ctx context.Context, actor, conversationCode, profileID string) error {
	if c.members == nil {
		return ErrMembersNotSupported
	}
	if _, err := c.Session(ctx, actor, conversationCode); err != nil {
		return err
	}
	if err := c.members.RemoveMember(ctx, actor, conversationCode, profileID); err != nil {
		return err
	}
	c.EmitInboxToConversationMembers(ctx, actor, conversationCode, "member_removed", MemberPayloadJSON(profileID))
	return nil
}
