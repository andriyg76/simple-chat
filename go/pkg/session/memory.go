package session

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/andriyg76/simple-chat/go/pkg/chatcore"
	"github.com/google/uuid"
)

type room struct {
	session chatcore.Session
	members map[string]struct{}
}

// Memory is an in-process Store, MembersStore, and MetadataStore for reference servers and tests.
type Memory struct {
	mu            sync.Mutex
	rooms         map[string]*room
	byParticipant map[string]map[string]chatcore.Session
}

func NewMemory() *Memory {
	return &Memory{
		rooms:         make(map[string]*room),
		byParticipant: make(map[string]map[string]chatcore.Session),
	}
}

func (m *Memory) List(_ context.Context, participant string, _ bool) ([]chatcore.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rooms := m.byParticipant[participant]
	out := make([]chatcore.Session, 0, len(rooms))
	for _, s := range rooms {
		out = append(out, s)
	}
	return out, nil
}

func (m *Memory) Create(_ context.Context, participant string) (chatcore.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byParticipant[participant] == nil {
		m.byParticipant[participant] = make(map[string]chatcore.Session)
	}
	n := len(m.byParticipant[participant]) + 1
	now := time.Now().UTC()
	s := chatcore.Session{
		ID:            uuid.NewString(),
		Title:         fmt.Sprintf("Chat %d", n),
		LastMessageAt: now,
	}
	m.rooms[s.ID] = &room{
		session: s,
		members: map[string]struct{}{participant: {}},
	}
	m.byParticipant[participant][s.ID] = s
	return s, nil
}

func (m *Memory) Get(_ context.Context, participant, code string) (chatcore.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[code]
	if !ok {
		return chatcore.Session{}, chatcore.ErrNotFound
	}
	if _, member := r.members[participant]; !member {
		return chatcore.Session{}, chatcore.ErrNotFound
	}
	return r.session, nil
}

func (m *Memory) UpdateTitle(_ context.Context, actorParticipant, conversationCode, title string) (chatcore.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[conversationCode]
	if !ok {
		return chatcore.Session{}, chatcore.ErrNotFound
	}
	if _, member := r.members[actorParticipant]; !member {
		return chatcore.Session{}, chatcore.ErrNotFound
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return chatcore.Session{}, chatcore.ErrTitleRequired
	}
	r.session.Title = title
	m.propagateSession(r)
	return r.session, nil
}

func (m *Memory) propagateSession(r *room) {
	s := r.session
	for profileID := range r.members {
		if m.byParticipant[profileID] == nil {
			m.byParticipant[profileID] = make(map[string]chatcore.Session)
		}
		m.byParticipant[profileID][s.ID] = s
	}
}

func (m *Memory) ListMembers(_ context.Context, actorParticipant, conversationCode string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[conversationCode]
	if !ok {
		return nil, chatcore.ErrNotFound
	}
	if _, member := r.members[actorParticipant]; !member {
		return nil, chatcore.ErrNotFound
	}
	out := make([]string, 0, len(r.members))
	for id := range r.members {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

func (m *Memory) AddMember(_ context.Context, actorParticipant, conversationCode, profileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[conversationCode]
	if !ok {
		return chatcore.ErrNotFound
	}
	if _, member := r.members[actorParticipant]; !member {
		return chatcore.ErrNotFound
	}
	if profileID == "" {
		return chatcore.ErrMemberNotFound
	}
	if _, exists := r.members[profileID]; exists {
		return chatcore.ErrMemberExists
	}
	r.members[profileID] = struct{}{}
	if m.byParticipant[profileID] == nil {
		m.byParticipant[profileID] = make(map[string]chatcore.Session)
	}
	m.byParticipant[profileID][conversationCode] = r.session
	return nil
}

func (m *Memory) RemoveMember(_ context.Context, actorParticipant, conversationCode, profileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[conversationCode]
	if !ok {
		return chatcore.ErrNotFound
	}
	if _, member := r.members[actorParticipant]; !member {
		return chatcore.ErrNotFound
	}
	if _, exists := r.members[profileID]; !exists {
		return chatcore.ErrMemberNotFound
	}
	if len(r.members) <= 1 {
		return chatcore.ErrLastMember
	}
	delete(r.members, profileID)
	if rooms := m.byParticipant[profileID]; rooms != nil {
		delete(rooms, conversationCode)
	}
	return nil
}
