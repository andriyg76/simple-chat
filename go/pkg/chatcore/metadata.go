package chatcore

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrTitleRequired         = errors.New("title_required")
	ErrMetadataNotSupported  = errors.New("metadata_not_supported")
)

// MetadataStore updates conversation metadata (title). Persistence is product-specific.
type MetadataStore interface {
	UpdateTitle(ctx context.Context, actorParticipant, conversationCode, title string) (Session, error)
}

func (c *Core) UpdateTitle(ctx context.Context, actor, conversationCode, title string) (Session, error) {
	if c.metadata == nil {
		return Session{}, ErrMetadataNotSupported
	}
	if _, err := c.Session(ctx, actor, conversationCode); err != nil {
		return Session{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return Session{}, ErrTitleRequired
	}
	session, err := c.metadata.UpdateTitle(ctx, actor, conversationCode, title)
	if err != nil {
		return Session{}, err
	}
	c.EmitInboxToConversationMembers(ctx, actor, conversationCode, "conversation_updated", TitleUpdatedJSON(title))
	return session, nil
}
