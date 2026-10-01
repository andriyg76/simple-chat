package chathttp

import (
	"context"
	"errors"

	"github.com/agushuley/simple-chat/go/pkg/chat"
	"github.com/agushuley/simple-chat/go/pkg/chatcore"
	"github.com/agushuley/simple-chat/go/pkg/msgstore"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type coreStore struct {
	chat     *chat.Service
	messages *msgstore.Store
}

func profileID(value string) (primitive.ObjectID, error) { return primitive.ObjectIDFromHex(value) }

func coreSession(c *chat.Conversation) chatcore.Session {
	return chatcore.Session{ID: c.Code, Title: c.Title, LastMessageAt: c.LastMessageAt, ReviewSummary: c.ReviewSummary}
}

func (s coreStore) List(ctx context.Context, participant string, archived bool) ([]chatcore.Session, error) {
	id, err := profileID(participant)
	if err != nil {
		return nil, err
	}
	items, err := s.chat.ListConversations(ctx, id, archived)
	if err != nil {
		return nil, err
	}
	result := make([]chatcore.Session, 0, len(items))
	for i := range items {
		result = append(result, coreSession(&items[i]))
	}
	return result, nil
}

func (s coreStore) Create(ctx context.Context, participant string) (chatcore.Session, error) {
	id, err := profileID(participant)
	if err != nil {
		return chatcore.Session{}, err
	}
	c, err := s.chat.CreateConversation(ctx, id)
	if err != nil {
		return chatcore.Session{}, err
	}
	return coreSession(c), nil
}

func (s coreStore) Get(ctx context.Context, participant, code string) (chatcore.Session, error) {
	id, err := profileID(participant)
	if err != nil {
		return chatcore.Session{}, err
	}
	c, err := s.chat.Conversation(ctx, id, code)
	if err != nil {
		if errors.Is(err, chat.ErrConversationNotFound) {
			return chatcore.Session{}, chatcore.ErrNotFound
		}
		return chatcore.Session{}, err
	}
	return coreSession(c), nil
}

func (s coreStore) Touch(ctx context.Context, participant, code string) error {
	id, err := profileID(participant)
	if err != nil {
		return err
	}
	c, err := s.chat.Conversation(ctx, id, code)
	if err != nil {
		if errors.Is(err, chat.ErrConversationNotFound) {
			return chatcore.ErrNotFound
		}
		return err
	}
	return s.chat.TouchConversation(ctx, c)
}

func (s coreStore) Save(ctx context.Context, participant, code, sender, text string) error {
	if _, err := s.Get(ctx, participant, code); err != nil {
		return err
	}
	direction := msgstore.DirectionIn
	if sender == "server" {
		direction = msgstore.DirectionOut
	}
	return s.messages.SaveChecked(ctx, "chat:"+code, msgstore.Message{Direction: direction, Body: text})
}
