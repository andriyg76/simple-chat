package chathttp

import "sync"

// InboxPresence tracks open inbox SSE connections per profile on this host.
type InboxPresence struct {
	mu    sync.Mutex
	count map[string]int
}

func NewInboxPresence() *InboxPresence {
	return &InboxPresence{count: make(map[string]int)}
}

func (p *InboxPresence) Connect(profileID string) {
	p.mu.Lock()
	p.count[profileID]++
	p.mu.Unlock()
}

func (p *InboxPresence) Disconnect(profileID string) {
	p.mu.Lock()
	n := p.count[profileID]
	if n <= 1 {
		delete(p.count, profileID)
	} else {
		p.count[profileID] = n - 1
	}
	p.mu.Unlock()
}

func (p *InboxPresence) IsConnected(profileID string) bool {
	p.mu.Lock()
	connected := p.count[profileID] > 0
	p.mu.Unlock()
	return connected
}
