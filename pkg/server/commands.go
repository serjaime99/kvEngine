package server

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/serjaime99/kvEngine/pkg/resp"
	"github.com/serjaime99/kvEngine/pkg/storage"
)

// Dispatcher executes parsed commands against the storage engine.
type Dispatcher struct {
	engine *storage.Engine
	aof    *storage.AOFWriter
}

func NewDispatcher(engine *storage.Engine, aof *storage.AOFWriter) *Dispatcher {
	return &Dispatcher{
		engine: engine,
		aof:    aof,
	}
}

// Execute processes a command array and returns the RESP response value.
func (d *Dispatcher) Execute(args []string) resp.Value {
	if len(args) == 0 {
		return resp.MakeError("ERR empty command")
	}

	cmd := strings.ToUpper(args[0])

	switch cmd {
	case "PING":
		if len(args) == 1 {
			return resp.MakeSimpleString("PONG")
		}
		return resp.MakeBulkString(args[1])

	case "SET":
		if len(args) < 3 {
			return resp.MakeError("ERR wrong number of arguments for 'set' command")
		}
		key := args[1]
		val := args[2]
		var ttlMs int64 = 0

		// Parse optional expiration flags: EX seconds or PX milliseconds
		if len(args) >= 5 {
			opt := strings.ToUpper(args[3])
			num, err := strconv.ParseInt(args[4], 10, 64)
			if err != nil {
				return resp.MakeError("ERR value is not an integer or out of range")
			}
			if opt == "EX" {
				ttlMs = num * 1000
			} else if opt == "PX" {
				ttlMs = num
			}
		}

		d.engine.Set(key, val, ttlMs)
		if d.aof != nil {
			_ = d.aof.WriteCommand(args)
		}
		return resp.MakeSimpleString("OK")

	case "GET":
		if len(args) != 2 {
			return resp.MakeError("ERR wrong number of arguments for 'get' command")
		}
		val, found := d.engine.Get(args[1])
		if !found {
			return resp.MakeNullBulkString()
		}
		return resp.MakeBulkString(val)

	case "DEL":
		if len(args) < 2 {
			return resp.MakeError("ERR wrong number of arguments for 'del' command")
		}
		count := int64(0)
		for i := 1; i < len(args); i++ {
			if d.engine.Del(args[i]) {
				count++
			}
		}
		if d.aof != nil && count > 0 {
			_ = d.aof.WriteCommand(args)
		}
		return resp.MakeInteger(count)

	case "EXISTS":
		if len(args) < 2 {
			return resp.MakeError("ERR wrong number of arguments for 'exists' command")
		}
		count := int64(0)
		for i := 1; i < len(args); i++ {
			if d.engine.Exists(args[i]) {
				count++
			}
		}
		return resp.MakeInteger(count)

	case "EXPIRE":
		if len(args) != 3 {
			return resp.MakeError("ERR wrong number of arguments for 'expire' command")
		}
		sec, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return resp.MakeError("ERR value is not an integer or out of range")
		}
		if d.engine.Expire(args[1], sec*1000) {
			if d.aof != nil {
				_ = d.aof.WriteCommand(args)
			}
			return resp.MakeInteger(1)
		}
		return resp.MakeInteger(0)

	case "TTL":
		if len(args) != 2 {
			return resp.MakeError("ERR wrong number of arguments for 'ttl' command")
		}
		ttlMs := d.engine.TTL(args[1])
		if ttlMs > 0 {
			return resp.MakeInteger(ttlMs / 1000) // Return seconds to match Redis standard
		}
		return resp.MakeInteger(ttlMs)

	case "KEYS":
		pattern := "*"
		if len(args) >= 2 {
			pattern = args[1]
		}
		keys := d.engine.Keys(pattern)
		items := make([]resp.Value, len(keys))
		for i, k := range keys {
			items[i] = resp.MakeBulkString(k)
		}
		return resp.MakeArray(items)

	case "DBSIZE":
		return resp.MakeInteger(int64(d.engine.Len()))

	case "INFO":
		infoText := fmt.Sprintf("# Server\r\nkvengine_version:1.0.0\r\ntotal_keys:%d\r\n", d.engine.Len())
		return resp.MakeBulkString(infoText)

	case "COMMAND":
		// Needed for modern redis-cli handshakes
		return resp.MakeArray([]resp.Value{})

	default:
		return resp.MakeError(fmt.Sprintf("ERR unknown command '%s'", cmd))
	}
}
