# kvEngine: High-Performance Redis-Compatible In-Memory Key-Value Store

A high-throughput, distributed-ready in-memory key-value database engine implemented in Go. Features raw TCP socket communication with the Redis Serialization Protocol (RESP), 64-stripe sharded locking to eliminate mutex contention, active probabilistic TTL garbage collection, and crash-safe Append-Only File (AOF) persistence.

---

## Key Architectural Highlights

```
                          CLIENTS (redis-cli / drivers)
                                     │ (TCP raw bytes)
                                     ▼
                     ┌────────────────────────────────┐
                     │     RESP Streaming Parser      │
                     └────────────────────────────────┘
                                     │ (Parsed Command)
                                     ▼
                     ┌────────────────────────────────┐
                     │       Command Dispatcher       │
                     └───────┬────────────────┬───────┘
                             │                │
          ┌──────────────────┘                └─────────────────┐
          ▼                                                     ▼
┌───────────────────────────────┐              ┌─────────────────────────────────┐
│     64-Shard Striped Store    │              │       AOF Persistence Log       │
│  [Shard 0]  ...  [Shard 63]   │              │     (fsync durability log)      │
└───────────────┬───────────────┘              └─────────────────────────────────┘
                ▲
                │ (Active sweep 10Hz)
┌───────────────┴───────────────┐
│  Probabilistic Janitor (TTL)  │
└───────────────────────────────┘
```

---

## Core Engineering Decisions

### 1. Lock-Striped Sharding (64 Independent Partitions)
A naive in-memory database uses a single global mutex wrapping a hash table. Under high concurrency, hundreds of client goroutines fight for that single lock, causing severe CPU lock contention.
`kvEngine` hashes incoming keys using the **64-bit FNV-1a algorithm** and routes operations to one of **64 independent shards**, each protected by its own `sync.RWMutex`. Parallel requests touching different keys execute with zero mutex lock stalls.

### 2. Native RESP Wire Protocol Parser
Instead of bulky JSON or HTTP, `kvEngine` implements a zero-allocation streaming parser for the **Redis Serialization Protocol (RESP)**. It natively parses:
- Simple Strings (`+`)
- Errors (`-`)
- Integers (`:`)
- Bulk Strings (`$`)
- Command Arrays (`*`)

This allows `kvEngine` to communicate directly with official Redis CLI clients (`redis-cli`) and standard Redis language drivers (Python `redis-py`, Node `ioredis`, Go `go-redis`).

### 3. Active + Passive Expiration Engine
- **Passive Expiration:** Expired keys requested by clients are lazily removed on-the-fly during read operations.
- **Probabilistic Active Cleaner:** A background worker runs at 10Hz sampling 20 random keys with deadlines per shard. If more than 25% are expired, it repeats immediately, preventing dead keys from consuming memory.

### 4. Append-Only File (AOF) Crash Durability
All mutating operations (`SET`, `DEL`, `EXPIRE`) are written synchronously to an append-only transaction log on disk. On restart, the engine automatically replays the AOF log to reconstruct state in memory.

---

## Benchmarks

Benchmarked on Linux (`Intel Core i5-3320M @ 2.60GHz`):

| Operation | Throughput | Latency |
| :--- | :--- | :--- |
| **Engine `Set` (Sequential)** | **~1,760,000 ops/sec** | **567 ns/op** |
| **Engine `ParallelSet` (Multi-core)** | **~752,000 ops/sec** | **1,328 ns/op** |
| **Race Detector (`-race`)** | **100% Clean** | **0 Data Races** |

---

## Directory Structure

```
kvEngine/
├── cmd/
│   └── server/
│       └── main.go       # Server entrypoint with graceful shutdown & signal handling
├── pkg/
│   ├── resp/
│   │   ├── types.go      # RESP protocol data types and byte serializer
│   │   ├── parser.go     # Zero-copy streaming RESP protocol parser
│   │   └── writer.go     # Buffered socket serializer
│   ├── storage/
│   │   ├── entry.go      # In-memory entry & TTL timestamps
│   │   ├── engine.go     # 64-shard lock-striped concurrent storage
│   │   ├── ttl.go        # Active probabilistic TTL garbage collector
│   │   └── aof.go        # Append-Only File persistence and crash replay
│   └── server/
│       ├── commands.go   # Command dispatcher (PING, SET, GET, DEL, etc.)
│       └── tcp.go        # Multi-client concurrent TCP listener
├── tests/
│   ├── resp_test.go      # Wire protocol unit tests
│   ├── storage_test.go   # Concurrency, race detector, & throughput tests
│   └── aof_test.go       # Crash recovery & persistence tests
├── Makefile              # Build, test (-race), benchmark, and run automation
├── go.mod
└── README.md
```

---

## Getting Started

### 1. Build and Run
```bash
make run
```
The server binds to port `6379` by default.

### 2. Connect via `redis-cli`
Open another terminal:
```bash
redis-cli -p 6379
127.0.0.1:6379> PING
PONG
127.0.0.1:6379> SET user:100 "Ana" EX 60
OK
127.0.0.1:6379> GET user:100
"Ana"
127.0.0.1:6379> TTL user:100
(integer) 58
```

### 3. Run Automated Tests
```bash
make test
```

### 4. Run Benchmarks
```bash
make bench
```
