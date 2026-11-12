package tests

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/serjaime99/kvEngine/pkg/storage"
)

func TestStorage_BasicOperations(t *testing.T) {
	engine := storage.NewEngine()

	// SET and GET
	engine.Set("user:1", "alice", 0)
	val, ok := engine.Get("user:1")
	if !ok || val != "alice" {
		t.Fatalf("expected alice, got %s (ok=%v)", val, ok)
	}

	// EXISTS
	if !engine.Exists("user:1") {
		t.Fatalf("expected user:1 to exist")
	}

	// DEL
	if !engine.Del("user:1") {
		t.Fatalf("expected DEL to succeed")
	}
	if engine.Exists("user:1") {
		t.Fatalf("expected user:1 to be deleted")
	}
}

func TestStorage_TTLPassiveExpiration(t *testing.T) {
	engine := storage.NewEngine()

	// Set key with 50ms expiration
	engine.Set("tempKey", "ephemeral", 50)

	val, ok := engine.Get("tempKey")
	if !ok || val != "ephemeral" {
		t.Fatalf("expected key to exist initially")
	}

	// Sleep past expiration
	time.Sleep(70 * time.Millisecond)

	_, ok = engine.Get("tempKey")
	if ok {
		t.Fatalf("expected key to have expired passively")
	}
}

func TestStorage_TTLActiveCleaner(t *testing.T) {
	engine := storage.NewEngine()
	stopChan := make(chan struct{})
	go engine.StartTTLWorker(stopChan)
	defer close(stopChan)

	// Insert 200 keys with 40ms TTL
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("shortlived:%d", i)
		engine.Set(key, "data", 40)
	}

	if engine.Len() != 200 {
		t.Fatalf("expected 200 keys, got %d", engine.Len())
	}

	// Wait for TTL expiration + background cleaner sweep
	time.Sleep(250 * time.Millisecond)

	remaining := engine.Len()
	if remaining != 0 {
		t.Fatalf("expected active cleaner to sweep all keys, but %d remain", remaining)
	}
}

func TestStorage_HighConcurrencyRaceFree(t *testing.T) {
	engine := storage.NewEngine()
	numWorkers := 50
	opsPerWorker := 500

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for w := 0; w < numWorkers; w++ {
		workerID := w
		go func() {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				key := fmt.Sprintf("key:%d:%d", workerID, i%20)
				// Write
				engine.Set(key, "concurrency-test", 0)
				// Read
				_, _ = engine.Get(key)
				// Occasional Delete
				if i%5 == 0 {
					engine.Del(key)
				}
			}
		}()
	}

	wg.Wait()
}

func BenchmarkEngine_Set(b *testing.B) {
	engine := storage.NewEngine()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Set("benchmarkKey", "benchmarkValue", 0)
	}
}

func BenchmarkEngine_ParallelSet(b *testing.B) {
	engine := storage.NewEngine()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		idx := 0
		for pb.Next() {
			key := fmt.Sprintf("k:%d", idx%1000)
			engine.Set(key, "val", 0)
			idx++
		}
	})
}
