package storage

import (
	"time"
)

const (
	TTLCheckInterval = 100 * time.Millisecond // 10Hz frequency
	SamplePerShard   = 20                     // Number of keys to sample per cycle
	ExpiredThreshold = 0.25                   // Repeat cycle if >25% expired
)

// StartTTLWorker runs the probabilistic background active expiration algorithm.
func (e *Engine) StartTTLWorker(stopChan <-chan struct{}) {
	ticker := time.NewTicker(TTLCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			e.activeExpireCycle()
		}
	}
}

// activeExpireCycle sweeps expired keys across shards using probabilistic sampling.
func (e *Engine) activeExpireCycle() {
	now := CurrentTimeMs()

	for _, shard := range e.shards {
		// Run sampling loop on the shard
		for {
			shard.Lock()
			totalSampled := 0
			totalExpired := 0

			// Iterate map (Go maps have randomized iteration order by specification)
			for key, entry := range shard.entries {
				if entry.ExpiresAt > 0 {
					totalSampled++
					if entry.IsExpired(now) {
						delete(shard.entries, key)
						totalExpired++
					}
				}
				if totalSampled >= SamplePerShard {
					break
				}
			}
			shard.Unlock()

			// If no keys had TTL, or fewer than 25% were expired, break to next shard
			if totalSampled == 0 || float64(totalExpired)/float64(totalSampled) < ExpiredThreshold {
				break
			}
		}
	}
}
