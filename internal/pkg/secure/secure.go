// Package secure generates random identifiers and the digests used to store
// secrets such as sessions, verification codes and invitations.
package secure

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// NewID returns a random 32-byte identifier encoded as hex.
func NewID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Hash returns the SHA-256 digest of v.
func Hash(v string) string {
	b := sha256.Sum256([]byte(v))
	return hex.EncodeToString(b[:])
}
