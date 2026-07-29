package ingest

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// GenerateKey creates a new ingest API key. It returns the raw key (which must
// be shown to the user exactly once and never stored) and its SHA-256 hex hash
// (which is stored in the database and used for subsequent lookups).
//
// The raw key format is: lum_live_<base64url(32 random bytes)>
func GenerateKey() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("ingest.GenerateKey: %w", err)
	}
	raw = "lum_live_" + base64.RawURLEncoding.EncodeToString(b)
	hash = HashKey(raw)
	return raw, hash, nil
}

// HashKey returns the SHA-256 hex digest of rawKey. This is used both when
// storing a newly-created key and when authenticating an incoming request.
func HashKey(rawKey string) string {
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:])
}
