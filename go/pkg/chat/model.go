package chat

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const Channel = "chat"

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type Profile struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	Code        string             `bson:"code" json:"code"`
	DisplayName string             `bson:"display_name" json:"displayName"`
	Role        Role               `bson:"role" json:"role"`
	CreatedAt   time.Time          `bson:"created_at" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updatedAt"`
}

type Identity struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	ProfileID primitive.ObjectID `bson:"profile_id"`
	Kind      string             `bson:"kind"`
	Value     string             `bson:"value"`
	CreatedAt time.Time          `bson:"created_at"`
	RevokedAt *time.Time         `bson:"revoked_at,omitempty"`
}

type Enrollment struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Digest    string             `bson:"digest"`
	ProfileID primitive.ObjectID `bson:"profile_id"`
	ExpiresAt time.Time          `bson:"expires_at"`
	UsedAt    *time.Time         `bson:"used_at,omitempty"`
	CreatedAt time.Time          `bson:"created_at"`
}

type Session struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Digest    string             `bson:"digest"`
	ProfileID primitive.ObjectID `bson:"profile_id"`
	ExpiresAt time.Time          `bson:"expires_at"`
	RevokedAt *time.Time         `bson:"revoked_at,omitempty"`
	CreatedAt time.Time          `bson:"created_at"`
}

type Conversation struct {
	Code          string             `bson:"conversation_code" json:"code"`
	ProfileID     primitive.ObjectID `bson:"profile_id" json:"-"`
	Title         string             `bson:"title" json:"title"`
	Channel       string             `bson:"channel" json:"channel"`
	ArchivedAt    *time.Time         `bson:"archived_at,omitempty" json:"archivedAt,omitempty"`
	LastMessageAt time.Time          `bson:"last_message_at" json:"lastMessageAt"`
	CreatedAt     time.Time          `bson:"created_at" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updated_at" json:"updatedAt"`
	ReviewSummary string             `bson:"review_summary,omitempty" json:"reviewSummary,omitempty"`
}
