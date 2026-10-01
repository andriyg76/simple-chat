package idcode_test

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/agushuley/simple-chat/go/pkg/idcode"
)

func TestRoundTrip(t *testing.T) {
	ids := []primitive.ObjectID{
		primitive.NewObjectID(),
		primitive.NewObjectID(),
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	}
	for _, id := range ids {
		code := idcode.Encode(id)
		if code == "" {
			t.Fatalf("Encode(%v) returned empty string", id)
		}
		got, err := idcode.Decode(code)
		if err != nil {
			t.Fatalf("Decode(%q) error: %v", code, err)
		}
		if got != id {
			t.Errorf("round-trip mismatch: got %v, want %v (code=%q)", got, id, code)
		}
	}
}

func TestDecodeInvalid(t *testing.T) {
	cases := []string{
		"",
		"0",       // '0' not in base58 alphabet
		"O",       // 'O' not in base58 alphabet
		"IIIIIII", // 'I' not in base58 alphabet
		"!@#$",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // way too long
	}
	for _, c := range cases {
		if _, err := idcode.Decode(c); err == nil {
			t.Errorf("Decode(%q) expected error, got nil", c)
		}
	}
}

func TestEncodeDistinct(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 100; i++ {
		code := idcode.Encode(primitive.NewObjectID())
		if _, dup := seen[code]; dup {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = struct{}{}
	}
}
