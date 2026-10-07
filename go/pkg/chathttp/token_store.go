package chathttp

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Profile is the chat participant identity returned at enrollment.
type Profile struct {
	Code        string `json:"code"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

type tokenRecord struct {
	profileID   string
	displayName string
	role        string
	issuedAt    time.Time
}

// TokenStore issues bearer tokens per profile with optional rotation grace.
type TokenStore struct {
	mu            sync.Mutex
	byToken       map[string]tokenRecord
	primary       map[string]string // profileID -> current token
	graceUntil    map[string]time.Time
	expiresIn     time.Duration
	graceDuration time.Duration
}

func NewTokenStore(expiresIn, grace time.Duration) *TokenStore {
	if expiresIn <= 0 {
		expiresIn = 30 * 24 * time.Hour
	}
	if grace <= 0 {
		grace = 2 * time.Minute
	}
	return &TokenStore{
		byToken:       make(map[string]tokenRecord),
		primary:       make(map[string]string),
		graceUntil:    make(map[string]time.Time),
		expiresIn:     expiresIn,
		graceDuration: grace,
	}
}

// InstallDevToken seeds a fixed token (reference harness).
func (s *TokenStore) InstallDevToken(token, profileID, displayName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byToken[token] = tokenRecord{profileID: profileID, displayName: displayName, role: "user", issuedAt: time.Now()}
	s.primary[profileID] = token
}

func (s *TokenStore) Issue(profileID, displayName, role string) (string, int64) {
	if role == "" {
		role = "user"
	}
	if displayName == "" {
		displayName = profileID
	}
	token := newSecretToken()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byToken[token] = tokenRecord{profileID: profileID, displayName: displayName, role: role, issuedAt: time.Now()}
	s.primary[profileID] = token
	return token, int64(s.expiresIn.Seconds())
}

func (s *TokenStore) Resolve(token string) (Profile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if until, ok := s.graceUntil[token]; ok {
		if now.After(until) {
			delete(s.graceUntil, token)
			delete(s.byToken, token)
		}
	}
	rec, ok := s.byToken[token]
	if !ok {
		return Profile{}, false
	}
	return Profile{Code: rec.profileID, DisplayName: rec.displayName, Role: rec.role}, true
}

// Rotate mints a new token for the profile; old token stays valid until grace ends.
func (s *TokenStore) Rotate(profileID string) (token string, rotationID string, expiresIn int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.primary[profileID]
	rec := s.byToken[old]
	if rec.profileID == "" {
		rec = tokenRecord{profileID: profileID, displayName: profileID, role: "user"}
	}
	token = newSecretToken()
	rotationID = newSecretToken()
	s.byToken[token] = tokenRecord{
		profileID:   profileID,
		displayName: rec.displayName,
		role:        rec.role,
		issuedAt:    time.Now(),
	}
	if old != "" {
		s.graceUntil[old] = time.Now().Add(s.graceDuration)
	}
	s.primary[profileID] = token
	return token, rotationID, int64(s.expiresIn.Seconds())
}

func (s *TokenStore) ProfileForID(profileID string) Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	tok := s.primary[profileID]
	if tok == "" {
		return Profile{Code: profileID, DisplayName: profileID, Role: "user"}
	}
	rec := s.byToken[tok]
	return Profile{Code: rec.profileID, DisplayName: rec.displayName, Role: rec.role}
}

func newSecretToken() string {
	var b [24]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
