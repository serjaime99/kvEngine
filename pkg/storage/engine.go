package storage

import (
	"path/filepath"
	"strings"
	"sync"
)

const NumShards = 64

// Shard holds a partition of the keyspace protected by its own RWMutex.
type Shard struct {
	sync.RWMutex
	entries map[string]Entry
}

// Engine implements a lock-striped concurrent key-value store.
type Engine struct {
	shards [NumShards]*Shard
	aof    *AOFWriter
}

// NewEngine initializes the sharded engine and spawns the background active TTL cleaner.
func NewEngine() *Engine {
	e := &Engine{}
	for i := 0; i < NumShards; i++ {
		e.shards[i] = &Shard{
			entries: make(map[string]Entry),
		}
	}
	return e
}

// SetAOF attaches an AOF persistence layer to the engine.
func (e *Engine) SetAOF(aof *AOFWriter) {
	e.aof = aof
}

// fnv64 computes a 64-bit FNV-1a hash of a string key.
func fnv64(key string) uint64 {
	var hash uint64 = 14695981039346656037
	for i := 0; i < len(key); i++ {
		hash ^= uint64(key[i])
		hash *= 1099511628211
	}
	return hash
}

func (e *Engine) getShard(key string) *Shard {
	idx := fnv64(key) % NumShards
	return e.shards[idx]
}

// Get retrieves a key's value. Performs passive TTL expiration if expired.
func (e *Engine) Get(key string) (string, bool) {
	shard := e.getShard(key)
	now := CurrentTimeMs()

	shard.RLock()
	entry, exists := shard.entries[key]
	shard.RUnlock()

	if !exists {
		return "", false
	}

	// Passive expiration check
	if entry.IsExpired(now) {
		shard.Lock()
		// Double-check under write lock in case another goroutine already acted
		if cur, ok := shard.entries[key]; ok && cur.IsExpired(now) {
			delete(shard.entries, key)
		}
		shard.Unlock()
		return "", false
	}

	return entry.Value, true
}

// Set inserts or updates a key. ttlMs == 0 means no expiration.
func (e *Engine) Set(key string, value string, ttlMs int64) {
	shard := e.getShard(key)
	var expiresAt int64
	if ttlMs > 0 {
		expiresAt = CurrentTimeMs() + ttlMs
	}

	shard.Lock()
	shard.entries[key] = Entry{
		Value:     value,
		ExpiresAt: expiresAt,
	}
	shard.Unlock()
}

// Del removes a key. Returns true if key was present and deleted.
func (e *Engine) Del(key string) bool {
	shard := e.getShard(key)
	shard.Lock()
	_, exists := shard.entries[key]
	if exists {
		delete(shard.entries, key)
	}
	shard.Unlock()
	return exists
}

// Exists checks if a key exists and has not expired.
func (e *Engine) Exists(key string) bool {
	_, found := e.Get(key)
	return found
}

// Expire sets a Time-To-Live on an existing key. Returns true if key exists.
func (e *Engine) Expire(key string, ttlMs int64) bool {
	shard := e.getShard(key)
	now := CurrentTimeMs()

	shard.Lock()
	defer shard.Unlock()

	entry, exists := shard.entries[key]
	if !exists || entry.IsExpired(now) {
		if exists {
			delete(shard.entries, key)
		}
		return false
	}

	entry.ExpiresAt = now + ttlMs
	shard.entries[key] = entry
	return true
}

// TTL returns remaining milliseconds until expiration:
// -1: Key exists with no expiration set.
// -2: Key does not exist or already expired.
func (e *Engine) TTL(key string) int64 {
	shard := e.getShard(key)
	now := CurrentTimeMs()

	shard.RLock()
	entry, exists := shard.entries[key]
	shard.RUnlock()

	if !exists {
		return -2
	}

	if entry.IsExpired(now) {
		shard.Lock()
		delete(shard.entries, key)
		shard.Unlock()
		return -2
	}

	return entry.TTLRemaining(now)
}

// Keys returns all active keys matching a pattern (e.g. "*" or prefix like "user:*").
func (e *Engine) Keys(pattern string) []string {
	now := CurrentTimeMs()
	var matches []string

	for _, shard := range e.shards {
		shard.RLock()
		for key, entry := range shard.entries {
			if !entry.IsExpired(now) {
				if pattern == "*" || matchesPattern(pattern, key) {
					matches = append(matches, key)
				}
			}
		}
		shard.RUnlock()
	}

	return matches
}

// Len returns total number of unexpired entries across all shards.
func (e *Engine) Len() int {
	now := CurrentTimeMs()
	total := 0
	for _, shard := range e.shards {
		shard.RLock()
		for _, entry := range shard.entries {
			if !entry.IsExpired(now) {
				total++
			}
		}
		shard.RUnlock()
	}
	return total
}

func matchesPattern(pattern, key string) bool {
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(key, prefix)
	}
	matched, _ := filepath.Match(pattern, key)
	return matched
}
