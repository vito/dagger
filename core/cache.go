package core

import (
	"crypto/sha256"
	"encoding/base64"

	"github.com/dagger/dagger/core/idproto"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// CacheVolume is a persistent volume with a globally scoped identifier.
type CacheVolume struct {
	ID   *idproto.ID `json:"id"`
	Keys []string    `json:"keys"`
}

var ErrInvalidCacheID = errors.New("invalid cache ID; create one using cacheVolume")

func NewCache(keys ...string) *CacheVolume {
	return &CacheVolume{Keys: keys}
}

func (cache *CacheVolume) Clone() *CacheVolume {
	cp := *cache
	cp.Keys = cloneSlice(cp.Keys)
	return &cp
}

func (cache *CacheVolume) Digest() (digest.Digest, error) {
	return stableDigest(cache)
}

// Sum returns a checksum of the cache tokens suitable for use as a cache key.
func (cache *CacheVolume) Sum() string {
	hash := sha256.New()
	for _, tok := range cache.Keys {
		_, _ = hash.Write([]byte(tok + "\x00"))
	}

	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

// CacheSharingMode is a string deriving from CacheSharingMode enum
// it can take values: SHARED, PRIVATE, LOCKED
type CacheSharingMode string

const (
	CacheSharingModeShared  CacheSharingMode = "SHARED"
	CacheSharingModePrivate CacheSharingMode = "PRIVATE"
	CacheSharingModeLocked  CacheSharingMode = "LOCKED"
)

func (cache *CacheVolume) WithKey(key string) *CacheVolume {
	cache = cache.Clone()
	cache.Keys = append(cache.Keys, key)
	return cache
}
