package storage

import "time"

// Entry holds the stored value and an optional expiration deadline.
type Entry struct {
	Value     string
	ExpiresAt int64 // Unix timestamp in milliseconds; 0 means no expiration
}

// IsExpired checks whether the entry has expired given the current unix millisecond timestamp.
func (e Entry) IsExpired(nowMs int64) bool {
	if e.ExpiresAt == 0 {
		return false
	}
	return nowMs >= e.ExpiresAt
}

// TTLRemaining returns remaining milliseconds until expiration, or -1 if no expiration, or -2 if expired.
func (e Entry) TTLRemaining(nowMs int64) int64 {
	if e.ExpiresAt == 0 {
		return -1 // No expiration set
	}
	remaining := e.ExpiresAt - nowMs
	if remaining <= 0 {
		return -2 // Already expired
	}
	return remaining
}

func CurrentTimeMs() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
