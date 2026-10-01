package idcode

import (
	"fmt"
	"math/big"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var (
	base     = big.NewInt(58)
	zero     = big.NewInt(0)
	alphaMap [256]int64

	pkgCache = NewCache(1000, time.Hour)
)

func init() {
	for i := range alphaMap {
		alphaMap[i] = -1
	}
	for i, c := range alphabet {
		alphaMap[c] = int64(i)
	}
}

// Encode encodes a MongoDB ObjectID into a short base58 string.
// Results are cached in the package-level cache (1000 entries, 1 h TTL).
func Encode(id primitive.ObjectID) string {
	return pkgCache.Encode(id)
}

// Decode decodes a base58 string back to a MongoDB ObjectID.
// Results are cached in the package-level cache (1000 entries, 1 h TTL).
func Decode(code string) (primitive.ObjectID, error) {
	return pkgCache.Decode(code)
}

// encode is the pure (uncached) implementation used by Cache internally.
func encode(id primitive.ObjectID) string {
	n := new(big.Int).SetBytes(id[:])
	buf := make([]byte, 0, 17)
	mod := new(big.Int)
	for n.Cmp(zero) > 0 {
		n.DivMod(n, base, mod)
		buf = append(buf, alphabet[mod.Int64()])
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

// decode is the pure (uncached) implementation used by Cache internally.
func decode(code string) (primitive.ObjectID, error) {
	if code == "" {
		return primitive.NilObjectID, fmt.Errorf("empty code")
	}
	n := new(big.Int)
	for _, c := range code {
		idx := alphaMap[c]
		if idx < 0 {
			return primitive.NilObjectID, fmt.Errorf("invalid character %q in code", c)
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(idx))
	}
	b := n.Bytes()
	if len(b) > 12 {
		return primitive.NilObjectID, fmt.Errorf("code too large for ObjectID")
	}
	var id primitive.ObjectID
	copy(id[12-len(b):], b)
	return id, nil
}
