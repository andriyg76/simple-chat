package chathttp

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/google/uuid"
)

type challengeStatus string

const (
	challengePending   challengeStatus = "pending"
	challengeCompleted challengeStatus = "completed"
)

type enrollmentChallenge struct {
	id        string
	secret    string
	expiresAt time.Time
	status    challengeStatus
	token     string
	expiresIn int64
	profile   Profile
}

// ChallengeStore holds in-flight enrollment mode B challenges.
type ChallengeStore struct {
	mu         sync.Mutex
	byID       map[string]*enrollmentChallenge
	bySecret   map[string]string
	defaultTTL time.Duration
}

func NewChallengeStore(ttl time.Duration) *ChallengeStore {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &ChallengeStore{byID: make(map[string]*enrollmentChallenge), bySecret: make(map[string]string), defaultTTL: ttl}
}

func (s *ChallengeStore) Create() *enrollmentChallenge {
	id := uuid.NewString()
	secret := newVerifySecret()
	ch := &enrollmentChallenge{
		id:        id,
		secret:    secret,
		expiresAt: time.Now().Add(s.defaultTTL),
		status:    challengePending,
	}
	s.mu.Lock()
	s.byID[id] = ch
	s.bySecret[secret] = id
	s.mu.Unlock()
	return ch
}

func (s *ChallengeStore) Get(id string) (*enrollmentChallenge, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.byID[id]
	if !ok {
		return nil, false
	}
	if ch.status == challengePending && time.Now().After(ch.expiresAt) {
		return ch, true
	}
	return ch, true
}

func (s *ChallengeStore) BySecret(secret string) (*enrollmentChallenge, bool) {
	s.mu.Lock()
	id := s.bySecret[secret]
	s.mu.Unlock()
	if id == "" {
		return nil, false
	}
	return s.Get(id)
}

func (s *ChallengeStore) Complete(ch *enrollmentChallenge, token string, expiresIn int64, profile Profile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch.status = challengeCompleted
	ch.token = token
	ch.expiresIn = expiresIn
	ch.profile = profile
}

func (ch *enrollmentChallenge) Expired() bool {
	return ch.status == challengePending && time.Now().After(ch.expiresAt)
}

func (ch *enrollmentChallenge) RemainingSeconds() int {
	if ch.Expired() {
		return 0
	}
	return int(time.Until(ch.expiresAt).Seconds())
}

func newVerifySecret() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
