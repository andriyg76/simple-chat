package idcode

import (
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type entry struct {
	id          primitive.ObjectID
	code        string
	createdAt   time.Time
	lastAccess  time.Time
	accessCount uint32
}

// Cache is a bounded bidirectional cache for ObjectID ↔ base58 code
// conversions. Entries expire after ttl (measured from creation). When the
// cache is full on insert, expired entries are evicted first; if still full,
// the least-frequently-used entry is removed (ties broken by oldest
// lastAccess). Thread-safe.
type Cache struct {
	mu      sync.Mutex
	byID    map[primitive.ObjectID]*entry
	byCode  map[string]*entry
	maxSize int
	ttl     time.Duration
}

// NewCache creates a Cache with the given capacity and TTL.
func NewCache(maxSize int, ttl time.Duration) *Cache {
	return &Cache{
		byID:    make(map[primitive.ObjectID]*entry, maxSize),
		byCode:  make(map[string]*entry, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Encode returns the base58 code for id, using a cached result when available.
func (c *Cache) Encode(id primitive.ObjectID) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if e, ok := c.byID[id]; ok {
		if now.Sub(e.createdAt) < c.ttl {
			e.lastAccess = now
			e.accessCount++
			return e.code
		}
		c.delete(e)
	}

	code := encode(id)
	c.store(id, code, now)
	return code
}

// Decode returns the ObjectID for code, using a cached result when available.
func (c *Cache) Decode(code string) (primitive.ObjectID, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if e, ok := c.byCode[code]; ok {
		if now.Sub(e.createdAt) < c.ttl {
			e.lastAccess = now
			e.accessCount++
			return e.id, nil
		}
		c.delete(e)
	}

	id, err := decode(code)
	if err != nil {
		return primitive.NilObjectID, err
	}
	c.store(id, code, now)
	return id, nil
}

// Len returns the number of live (non-expired) entries currently in the cache.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.byID)
}

func (c *Cache) store(id primitive.ObjectID, code string, now time.Time) {
	if len(c.byID) >= c.maxSize {
		c.evict(now)
	}
	e := &entry{
		id:          id,
		code:        code,
		createdAt:   now,
		lastAccess:  now,
		accessCount: 1,
	}
	c.byID[id] = e
	c.byCode[code] = e
}

func (c *Cache) delete(e *entry) {
	delete(c.byID, e.id)
	delete(c.byCode, e.code)
}

// evict removes expired entries first; if still at capacity, removes the
// least-frequently-used entry (ties broken by oldest lastAccess).
func (c *Cache) evict(now time.Time) {
	for _, e := range c.byID {
		if now.Sub(e.createdAt) >= c.ttl {
			c.delete(e)
		}
	}
	if len(c.byID) < c.maxSize {
		return
	}
	var victim *entry
	for _, e := range c.byID {
		if victim == nil ||
			e.accessCount < victim.accessCount ||
			(e.accessCount == victim.accessCount && e.lastAccess.Before(victim.lastAccess)) {
			victim = e
		}
	}
	if victim != nil {
		c.delete(victim)
	}
}
