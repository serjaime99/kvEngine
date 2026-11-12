package storage

import (
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/serjaime99/kvEngine/pkg/resp"
)

// AOFWriter handles durable disk logging for mutating operations.
type AOFWriter struct {
	mu   sync.Mutex
	file *os.File
}

// NewAOF opens or creates the Append-Only File for appending.
func NewAOF(path string) (*AOFWriter, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &AOFWriter{file: file}, nil
}

// WriteCommand logs a command as a standard RESP array to disk.
func (a *AOFWriter) WriteCommand(args []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	items := make([]resp.Value, len(args))
	for i, arg := range args {
		items[i] = resp.MakeBulkString(arg)
	}

	payload := resp.MakeArray(items).ToBytes()
	_, err := a.file.Write(payload)
	return err
}

// Sync commits pending buffered data to persistent storage (fsync).
func (a *AOFWriter) Sync() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.file.Sync()
}

// Close closes the underlying log file.
func (a *AOFWriter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.file.Close()
}

// ReplayAOF reads an existing AOF file and restores database state into memory.
func ReplayAOF(path string, engine *Engine) (int, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil // No existing file to replay
	}
	if err != nil {
		return 0, err
	}
	defer file.Close()

	parser := resp.NewParser(file)
	recoveredCount := 0

	for {
		val, err := parser.Parse()
		if err == io.EOF {
			break
		}
		if err != nil {
			return recoveredCount, err
		}

		if val.Type != resp.TypeArray || len(val.Array) == 0 {
			continue
		}

		cmd := strings.ToUpper(val.Array[0].Str)
		switch cmd {
		case "SET":
			if len(val.Array) >= 3 {
				key := val.Array[1].Str
				value := val.Array[2].Str
				var ttlMs int64 = 0
				if len(val.Array) >= 5 && strings.ToUpper(val.Array[3].Str) == "PX" {
					ttlMs, _ = strconv.ParseInt(val.Array[4].Str, 10, 64)
				}
				engine.Set(key, value, ttlMs)
				recoveredCount++
			}
		case "DEL":
			for i := 1; i < len(val.Array); i++ {
				engine.Del(val.Array[i].Str)
				recoveredCount++
			}
		case "EXPIRE":
			if len(val.Array) >= 3 {
				key := val.Array[1].Str
				sec, _ := strconv.ParseInt(val.Array[2].Str, 10, 64)
				engine.Expire(key, sec*1000)
				recoveredCount++
			}
		}
	}

	return recoveredCount, nil
}
