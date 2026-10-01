package msgstore

import (
	"testing"
	"time"
)

func TestFilterSince(t *testing.T) {
	base := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	msgs := []Message{
		{Body: "a", CreatedAt: base},
		{Body: "b", CreatedAt: base.Add(1 * time.Hour)},
		{Body: "c", CreatedAt: base.Add(2 * time.Hour)},
	}

	t.Run("zero since returns everything", func(t *testing.T) {
		got := filterSince(msgs, time.Time{})
		if len(got) != 3 {
			t.Errorf("filterSince() len = %d, want 3", len(got))
		}
	})

	t.Run("strict after excludes the boundary message", func(t *testing.T) {
		got := filterSince(msgs, base.Add(1*time.Hour))
		if len(got) != 1 || got[0].Body != "c" {
			t.Errorf("filterSince() = %v, want only [c]", got)
		}
	})

	t.Run("since after all messages returns nothing", func(t *testing.T) {
		got := filterSince(msgs, base.Add(10*time.Hour))
		if len(got) != 0 {
			t.Errorf("filterSince() len = %d, want 0", len(got))
		}
	})
}

func TestTailLimit(t *testing.T) {
	msgs := []Message{{Body: "a"}, {Body: "b"}, {Body: "c"}}

	t.Run("limit larger than slice returns everything", func(t *testing.T) {
		got := tailLimit(msgs, 10)
		if len(got) != 3 {
			t.Errorf("tailLimit() len = %d, want 3", len(got))
		}
	})

	t.Run("limit smaller than slice keeps the tail", func(t *testing.T) {
		got := tailLimit(msgs, 2)
		if len(got) != 2 || got[0].Body != "b" || got[1].Body != "c" {
			t.Errorf("tailLimit() = %v, want [b c]", got)
		}
	})

	t.Run("limit zero returns empty", func(t *testing.T) {
		got := tailLimit(msgs, 0)
		if len(got) != 0 {
			t.Errorf("tailLimit() len = %d, want 0", len(got))
		}
	})
}
