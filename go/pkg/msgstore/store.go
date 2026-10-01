package msgstore

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Direction string

const (
	DirectionIn  Direction = "in"
	DirectionOut Direction = "out"
)

// Message is one stored chat message (inbound or outbound), held inside a
// conversation's Messages array — see chatDoc.
type Message struct {
	Phone     string    `bson:"phone"`
	Direction Direction `bson:"direction"`
	Body      string    `bson:"body"`
	CreatedAt time.Time `bson:"created_at"`
}

type ProgressStep struct {
	At   time.Time `bson:"at"`
	Text string    `bson:"text"`
}

func (s *Store) StartProgress(ctx context.Context, convID, requestID string) {
	now := time.Now()
	entry := bson.M{"request_id": requestID, "started_at": now, "steps": []ProgressStep{}}
	_, err := s.col.UpdateOne(ctx, bson.M{"conv_id": convID}, bson.M{"$push": bson.M{"details.progress_log": bson.M{"$each": []bson.M{entry}, "$slice": -20}}, "$setOnInsert": bson.M{"created_at": now}}, options.Update().SetUpsert(true))
	if err != nil {
		slog.Warn("message store: start progress failed", "convId", convID, "err", err)
	}
}

func (s *Store) AppendProgress(ctx context.Context, convID, requestID, text string) {
	step := ProgressStep{At: time.Now(), Text: text}
	_, err := s.col.UpdateOne(ctx, bson.M{"conv_id": convID, "details.progress_log.request_id": requestID}, bson.M{"$push": bson.M{"details.progress_log.$.steps": step}})
	if err != nil {
		slog.Warn("message store: append progress failed", "convId", convID, "err", err)
	}
}

func (s *Store) FinishProgress(ctx context.Context, convID, requestID string) {
	now := time.Now()
	_, err := s.col.UpdateOne(ctx, bson.M{"conv_id": convID, "details.progress_log.request_id": requestID}, bson.M{"$set": bson.M{"details.progress_log.$.finished_at": now}})
	if err != nil {
		slog.Warn("message store: finish progress failed", "convId", convID, "err", err)
	}
}

// chatDoc is the per-conversation document: one row per convID, shared with
// checkpoint/convstate in the same conversation_sessions collection (each
// package $set-s only its own top-level fields). Messages holds only
// not-yet-reviewed messages — Flush empties it once they're folded into a
// checkpoint summary; LastMessageAt is denormalized and never reset by
// Flush, so activity discovery keeps working across a flush.
type chatDoc struct {
	ConvID        string    `bson:"conv_id"`
	Messages      []Message `bson:"messages,omitempty"`
	LastMessageAt time.Time `bson:"last_message_at"`
	CreatedAt     time.Time `bson:"created_at"`
	UpdatedAt     time.Time `bson:"updated_at"`
}

// Store persists and retrieves messages.
type Store struct {
	col *mongo.Collection
}

func NewStore(col *mongo.Collection) *Store {
	return &Store{col: col}
}

func (s *Store) EnsureIndexes(ctx context.Context) error {
	_, err := s.col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "conv_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

// Save appends a message to convID's pending buffer. Errors are logged but
// not propagated — message storage is best-effort and should not block
// message processing.
func (s *Store) Save(ctx context.Context, convID string, m Message) {
	if err := s.SaveChecked(ctx, convID, m); err != nil {
		slog.Warn("message store: save failed", "convId", convID, "err", err)
	}
}

// SaveChecked persists a chat message and reports storage failure to callers
// that must not announce delivery until the message is in history.
func (s *Store) SaveChecked(ctx context.Context, convID string, m Message) error {
	m.CreatedAt = time.Now()
	now := m.CreatedAt
	_, err := s.col.UpdateOne(ctx,
		bson.M{"conv_id": convID},
		bson.M{
			"$push":        bson.M{"messages": m},
			"$set":         bson.M{"last_message_at": now, "updated_at": now},
			"$setOnInsert": bson.M{"created_at": now},
		},
		options.Update().SetUpsert(true),
	)
	return err
}

// Flush empties convID's pending message buffer. Callers must only call
// this after the buffer's contents have been durably folded into a
// checkpoint summary — a failed checkpoint write should skip Flush so the
// raw messages survive for a later retry instead of being silently lost.
func (s *Store) Flush(ctx context.Context, convID string) error {
	_, err := s.col.UpdateOne(ctx,
		bson.M{"conv_id": convID},
		bson.M{"$set": bson.M{"messages": []Message{}, "updated_at": time.Now()}},
	)
	return err
}

// filterSince returns the messages created strictly after since (zero value
// = no lower bound), preserving order. Pure, split out from LoadHistory/
// CountSince for DB-free testing.
func filterSince(msgs []Message, since time.Time) []Message {
	var out []Message
	for _, m := range msgs {
		if since.IsZero() || m.CreatedAt.After(since) {
			out = append(out, m)
		}
	}
	return out
}

// tailLimit returns at most the last `limit` messages, preserving order.
// Pure, split out from LoadHistory for DB-free testing.
func tailLimit(msgs []Message, limit int) []Message {
	if len(msgs) > limit {
		return msgs[len(msgs)-limit:]
	}
	return msgs
}

// LoadHistory returns the last `limit` messages for convID created after
// `since` (zero value = no lower bound), oldest-first.
func (s *Store) LoadHistory(ctx context.Context, convID string, since time.Time, limit int) ([]*Message, error) {
	doc, err := s.findOne(ctx, convID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, nil
	}
	filtered := tailLimit(filterSince(doc.Messages, since), limit)
	msgs := make([]*Message, len(filtered))
	for i := range filtered {
		msgs[i] = &filtered[i]
	}
	return msgs, nil
}

// CountSince returns the number of messages for convID created after since
// (zero value = no lower bound) — uses the same strict-After filter as
// LoadHistory, so the count means exactly "how many messages seedHistory
// would load".
func (s *Store) CountSince(ctx context.Context, convID string, since time.Time) (int64, error) {
	doc, err := s.findOne(ctx, convID)
	if err != nil {
		return 0, err
	}
	if doc == nil {
		return 0, nil
	}
	return int64(len(filterSince(doc.Messages, since))), nil
}

// ChatActivity is one conversation's most recent message time.
type ChatActivity struct {
	ConvID        string
	LastMessageAt time.Time
}

// ListChatActivity returns the most recent message time per conversation,
// across every conversation ever seen — the durable source of truth for
// which conversations exist, used by the nightly reviewer to discover
// conversations that aren't currently resident in the in-memory
// ConversationStore (e.g. after a server restart). A plain projected Find —
// no aggregation needed, since last_message_at is denormalized onto each
// conversation's own single document.
func (s *Store) ListChatActivity(ctx context.Context) ([]ChatActivity, error) {
	cursor, err := s.col.Find(ctx, bson.M{"archived_at": bson.M{"$exists": false}},
		options.Find().SetProjection(bson.M{"conv_id": 1, "last_message_at": 1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var raw []struct {
		ConvID        string    `bson:"conv_id"`
		LastMessageAt time.Time `bson:"last_message_at"`
	}
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}
	activity := make([]ChatActivity, 0, len(raw))
	for _, r := range raw {
		activity = append(activity, ChatActivity{ConvID: r.ConvID, LastMessageAt: r.LastMessageAt})
	}
	return activity, nil
}

// LastMessageAt returns the denormalized last-message time for convID, or
// the zero time if the conversation has no stored document yet.
func (s *Store) LastMessageAt(ctx context.Context, convID string) (time.Time, error) {
	var doc struct {
		LastMessageAt time.Time `bson:"last_message_at"`
	}
	err := s.col.FindOne(ctx, bson.M{"conv_id": convID},
		options.FindOne().SetProjection(bson.M{"last_message_at": 1}),
	).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return doc.LastMessageAt, nil
}

func (s *Store) findOne(ctx context.Context, convID string) (*chatDoc, error) {
	var doc chatDoc
	err := s.col.FindOne(ctx, bson.M{"conv_id": convID}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}
