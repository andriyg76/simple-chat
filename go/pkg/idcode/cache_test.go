package idcode

import (
	"fmt"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCacheEncodeDecodeRoundTrip(t *testing.T) {
	c := NewCache(100, time.Minute)
	id := primitive.NewObjectID()

	code := c.Encode(id)
	if code == "" {
		t.Fatal("Encode returned empty string")
	}

	got, err := c.Decode(code)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if got != id {
		t.Errorf("round-trip mismatch: got %v, want %v", got, id)
	}
}

func TestCacheHitSkipsBigInt(t *testing.T) {
	c := NewCache(100, time.Minute)
	id := primitive.NewObjectID()

	code1 := c.Encode(id)
	code2 := c.Encode(id) // should come from cache
	if code1 != code2 {
		t.Errorf("second Encode returned different code: %q vs %q", code1, code2)
	}
	if c.Len() != 1 {
		t.Errorf("expected 1 cache entry after two Encodes of the same ID, got %d", c.Len())
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	c := NewCache(100, 10*time.Millisecond)
	id := primitive.NewObjectID()

	c.Encode(id)
	if c.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", c.Len())
	}

	time.Sleep(20 * time.Millisecond)

	// A fresh Encode triggers a lookup; the expired entry is removed first.
	c.Encode(id)
	// Entry was re-added, so still 1.
	if c.Len() != 1 {
		t.Errorf("expected 1 entry after re-encode post-expiry, got %d", c.Len())
	}
}

func TestCacheMaxSizeEvictsLFU(t *testing.T) {
	const max = 5
	c := NewCache(max, time.Minute)

	ids := make([]primitive.ObjectID, max)
	for i := range ids {
		ids[i] = primitive.NewObjectID()
		c.Encode(ids[i])
	}
	// Access ids[0] extra times so it has the highest count.
	for i := 0; i < 10; i++ {
		c.Encode(ids[0])
	}
	if c.Len() != max {
		t.Fatalf("expected %d entries, got %d", max, c.Len())
	}

	// Adding one more should evict the LFU entry (one of ids[1..4], each count=1).
	victim := primitive.NewObjectID()
	c.Encode(victim)
	if c.Len() != max {
		t.Errorf("expected %d entries after eviction, got %d", max, c.Len())
	}
	// ids[0] (high count) must still be present.
	code0 := encode(ids[0])
	if got := c.Encode(ids[0]); got != code0 {
		t.Errorf("high-frequency entry was wrongly evicted")
	}
}

func TestCacheDecodeInvalidCode(t *testing.T) {
	c := NewCache(100, time.Minute)
	if _, err := c.Decode("!invalid!"); err == nil {
		t.Error("expected error for invalid code")
	}
	if c.Len() != 0 {
		t.Errorf("failed decode should not populate cache, got %d entries", c.Len())
	}
}

func TestCacheConcurrent(t *testing.T) {
	c := NewCache(100, time.Minute)
	ids := make([]primitive.ObjectID, 20)
	for i := range ids {
		ids[i] = primitive.NewObjectID()
	}

	done := make(chan struct{})
	for g := 0; g < 10; g++ {
		go func(g int) {
			for i := 0; i < 50; i++ {
				id := ids[(g*7+i)%len(ids)]
				code := c.Encode(id)
				if got, err := c.Decode(code); err != nil || got != id {
					panic(fmt.Sprintf("concurrent round-trip failed: %v %v", got, err))
				}
			}
			done <- struct{}{}
		}(g)
	}
	for g := 0; g < 10; g++ {
		<-done
	}
}
