package chat

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agushuley/simple-chat/go/pkg/idcode"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Service struct {
	profiles, identities, enrollments, sessions, conversations *mongo.Collection
}

var ErrConversationNotFound = errors.New("conversation not found")

func NewService(profiles, identities, enrollments, sessions, conversations *mongo.Collection) *Service {
	return &Service{profiles: profiles, identities: identities, enrollments: enrollments, sessions: sessions, conversations: conversations}
}

func (s *Service) EnsureIndexes(ctx context.Context) error {
	models := []struct {
		c *mongo.Collection
		m []mongo.IndexModel
	}{
		{s.profiles, []mongo.IndexModel{{Keys: bson.D{{Key: "code", Value: 1}}, Options: options.Index().SetUnique(true)}}},
		{s.identities, []mongo.IndexModel{{Keys: bson.D{{Key: "kind", Value: 1}, {Key: "value", Value: 1}}, Options: options.Index().SetUnique(true)}}},
		{s.enrollments, []mongo.IndexModel{{Keys: bson.D{{Key: "digest", Value: 1}}, Options: options.Index().SetUnique(true)}, {Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)}}},
		{s.sessions, []mongo.IndexModel{{Keys: bson.D{{Key: "digest", Value: 1}}, Options: options.Index().SetUnique(true)}, {Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)}}},
		{s.conversations, []mongo.IndexModel{{Keys: bson.D{{Key: "conversation_code", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)}, {Keys: bson.D{{Key: "channel", Value: 1}, {Key: "profile_id", Value: 1}, {Key: "last_message_at", Value: -1}}}}},
	}
	for _, item := range models {
		if _, err := item.c.Indexes().CreateMany(ctx, item.m); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Bootstrap(ctx context.Context, displayName string) (*Profile, string, error) {
	n, err := s.profiles.CountDocuments(ctx, bson.D{})
	if err != nil {
		return nil, "", err
	}
	if n != 0 {
		return nil, "", fmt.Errorf("profiles already bootstrapped")
	}
	return s.CreateProfile(ctx, displayName, RoleAdmin)
}

// ResetBootstrapState removes all chat profiles and their credentials, chat
// channel conversations (including profile assignments), and enrollments so
// bootstrap can run again.
func (s *Service) ResetBootstrapState(ctx context.Context) (int64, error) {
	if _, err := s.enrollments.DeleteMany(ctx, bson.D{}); err != nil {
		return 0, err
	}
	if _, err := s.sessions.DeleteMany(ctx, bson.D{}); err != nil {
		return 0, err
	}
	if _, err := s.identities.DeleteMany(ctx, bson.D{}); err != nil {
		return 0, err
	}
	if _, err := s.conversations.DeleteMany(ctx, bson.M{"channel": Channel}); err != nil {
		return 0, err
	}
	result, err := s.profiles.DeleteMany(ctx, bson.D{})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

func (s *Service) CreateProfile(ctx context.Context, displayName string, role Role) (*Profile, string, error) {
	if strings.TrimSpace(displayName) == "" {
		return nil, "", fmt.Errorf("display name is required")
	}
	if role != RoleAdmin {
		role = RoleUser
	}
	now := time.Now()
	p := &Profile{ID: primitive.NewObjectID(), DisplayName: strings.TrimSpace(displayName), Role: role, CreatedAt: now, UpdatedAt: now}
	p.Code = idcode.Encode(p.ID)
	if _, err := s.profiles.InsertOne(ctx, p); err != nil {
		return nil, "", err
	}
	code, err := s.IssueEnrollment(ctx, p.ID)
	return p, code, err
}

// EnsureWhatsAppProfile lazily turns a legacy linked phone into a canonical
// profile. Identity uniqueness makes concurrent first messages converge on the
// same profile.
func (s *Service) EnsureWhatsAppProfile(ctx context.Context, phone, displayName string, role Role) (*Profile, error) {
	var identity Identity
	err := s.identities.FindOne(ctx, bson.M{"kind": "whatsapp_phone", "value": phone, "revoked_at": bson.M{"$exists": false}}).Decode(&identity)
	if err == nil {
		return s.Profile(ctx, identity.ProfileID)
	}
	if err != mongo.ErrNoDocuments {
		return nil, err
	}
	now := time.Now()
	if role != RoleAdmin {
		role = RoleUser
	}
	p := &Profile{ID: primitive.NewObjectID(), DisplayName: displayName, Role: role, CreatedAt: now, UpdatedAt: now}
	p.Code = idcode.Encode(p.ID)
	if _, err := s.profiles.InsertOne(ctx, p); err != nil {
		return nil, err
	}
	_, err = s.identities.InsertOne(ctx, Identity{ProfileID: p.ID, Kind: "whatsapp_phone", Value: phone, CreatedAt: now})
	if err == nil {
		return p, nil
	}
	if mongo.IsDuplicateKeyError(err) {
		if retryErr := s.identities.FindOne(ctx, bson.M{"kind": "whatsapp_phone", "value": phone, "revoked_at": bson.M{"$exists": false}}).Decode(&identity); retryErr == nil {
			return s.Profile(ctx, identity.ProfileID)
		}
	}
	return nil, err
}

func (s *Service) IssueEnrollment(ctx context.Context, profileID primitive.ObjectID) (string, error) {
	code, err := randomToken()
	if err != nil {
		return "", err
	}
	_, err = s.enrollments.InsertOne(ctx, Enrollment{Digest: digest(code), ProfileID: profileID, ExpiresAt: time.Now().Add(5 * time.Minute), CreatedAt: time.Now()})
	return code, err
}

func (s *Service) ExchangeEnrollment(ctx context.Context, code string) (*Profile, string, error) {
	now := time.Now()
	var e Enrollment
	err := s.enrollments.FindOneAndUpdate(ctx, bson.M{"digest": digest(code), "used_at": bson.M{"$exists": false}, "expires_at": bson.M{"$gt": now}}, bson.M{"$set": bson.M{"used_at": now}}, options.FindOneAndUpdate().SetReturnDocument(options.Before)).Decode(&e)
	if err == mongo.ErrNoDocuments {
		return nil, "", fmt.Errorf("invalid or expired enrollment code")
	}
	if err != nil {
		return nil, "", err
	}
	p, err := s.Profile(ctx, e.ProfileID)
	if err != nil {
		return nil, "", err
	}
	token, err := randomToken()
	if err != nil {
		return nil, "", err
	}
	_, err = s.sessions.InsertOne(ctx, Session{Digest: digest(token), ProfileID: p.ID, ExpiresAt: now.Add(30 * 24 * time.Hour), CreatedAt: now})
	return p, token, err
}

func (s *Service) Authenticate(ctx context.Context, token string) (*Profile, error) {
	var session Session
	err := s.sessions.FindOne(ctx, bson.M{"digest": digest(token), "revoked_at": bson.M{"$exists": false}, "expires_at": bson.M{"$gt": time.Now()}}).Decode(&session)
	if err == mongo.ErrNoDocuments {
		return nil, fmt.Errorf("unauthorized")
	}
	if err != nil {
		return nil, err
	}
	return s.Profile(ctx, session.ProfileID)
}

func (s *Service) Profile(ctx context.Context, id primitive.ObjectID) (*Profile, error) {
	var p Profile
	err := s.profiles.FindOne(ctx, bson.M{"_id": id}).Decode(&p)
	if err == mongo.ErrNoDocuments {
		return nil, fmt.Errorf("profile not found")
	}
	return &p, err
}

func (s *Service) ProfileByCode(ctx context.Context, code string) (*Profile, error) {
	var p Profile
	err := s.profiles.FindOne(ctx, bson.M{"code": code}).Decode(&p)
	if err == mongo.ErrNoDocuments {
		return nil, fmt.Errorf("profile not found")
	}
	return &p, err
}

func (s *Service) ListProfiles(ctx context.Context) ([]Profile, error) {
	cur, err := s.profiles.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "display_name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var profiles []Profile
	return profiles, cur.All(ctx, &profiles)
}

func (s *Service) RevokeSessions(ctx context.Context, profileID primitive.ObjectID) error {
	now := time.Now()
	_, err := s.sessions.UpdateMany(ctx, bson.M{"profile_id": profileID, "revoked_at": bson.M{"$exists": false}}, bson.M{"$set": bson.M{"revoked_at": now}})
	return err
}

func (s *Service) CreateConversation(ctx context.Context, profileID primitive.ObjectID) (*Conversation, error) {
	now := time.Now()
	id := primitive.NewObjectID()
	c := &Conversation{Code: idcode.Encode(id), ProfileID: profileID, Channel: Channel, Title: "Chat " + now.Format("2006-01-02 15:04"), CreatedAt: now, UpdatedAt: now, LastMessageAt: now}
	_, err := s.conversations.InsertOne(ctx, c)
	return c, err
}

func (s *Service) ListConversations(ctx context.Context, profileID primitive.ObjectID, archived bool) ([]Conversation, error) {
	filter := bson.M{"channel": Channel, "profile_id": profileID}
	if archived {
		filter["archived_at"] = bson.M{"$exists": true}
	} else {
		filter["archived_at"] = bson.M{"$exists": false}
	}
	cur, err := s.conversations.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "last_message_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var result []Conversation
	return result, cur.All(ctx, &result)
}

func (s *Service) Conversation(ctx context.Context, profileID primitive.ObjectID, code string) (*Conversation, error) {
	var c Conversation
	err := s.conversations.FindOne(ctx, bson.M{"conversation_code": code, "channel": Channel, "profile_id": profileID, "archived_at": bson.M{"$exists": false}}).Decode(&c)
	if err == mongo.ErrNoDocuments {
		return nil, ErrConversationNotFound
	}
	return &c, err
}

func (s *Service) TouchConversation(ctx context.Context, c *Conversation) error {
	now := time.Now()
	_, err := s.conversations.UpdateOne(ctx, bson.M{"conversation_code": c.Code, "profile_id": c.ProfileID}, bson.M{"$set": bson.M{"last_message_at": now, "updated_at": now}})
	return err
}

func (s *Service) UpdateTitle(ctx context.Context, profileID primitive.ObjectID, code, title string) error {
	if strings.TrimSpace(title) == "" {
		return nil
	}
	_, err := s.conversations.UpdateOne(ctx, bson.M{"conversation_code": code, "profile_id": profileID, "channel": Channel}, bson.M{"$set": bson.M{"title": title, "updated_at": time.Now()}})
	return err
}

func (s *Service) Archive(ctx context.Context, profileID primitive.ObjectID, code, summary string) error {
	now := time.Now()
	_, err := s.conversations.UpdateOne(ctx, bson.M{"conversation_code": code, "profile_id": profileID, "channel": Channel}, bson.M{"$set": bson.M{"archived_at": now, "review_summary": summary, "updated_at": now}})
	return err
}

func (s *Service) BindConversation(ctx context.Context, profileID primitive.ObjectID, code string) error {
	now := time.Now()
	result, err := s.conversations.UpdateOne(ctx, bson.M{"conversation_code": code, "channel": Channel}, bson.M{"$set": bson.M{"profile_id": profileID, "updated_at": now}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("conversation not found")
	}
	return nil
}

func Scopes(role Role) []string {
	if role == RoleAdmin {
		return []string{"league:read", "league:write", "game:write", "admin", "skill:tune", "user"}
	}
	return []string{"league:read", "game:write", "user"}
}
func digest(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }
func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
