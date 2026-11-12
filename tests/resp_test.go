package tests

import (
	"bytes"
	"testing"

	"github.com/serjaime99/kvEngine/pkg/resp"
)

func TestRESPParser_SimpleString(t *testing.T) {
	input := "+OK\r\n"
	p := resp.NewParser(bytes.NewBufferString(input))
	val, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if val.Type != resp.TypeSimpleString || val.Str != "OK" {
		t.Fatalf("expected +OK, got %s", val.String())
	}
}

func TestRESPParser_Integer(t *testing.T) {
	input := ":1024\r\n"
	p := resp.NewParser(bytes.NewBufferString(input))
	val, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if val.Type != resp.TypeInteger || val.Num != 1024 {
		t.Fatalf("expected :1024, got %s", val.String())
	}
}

func TestRESPParser_BulkString(t *testing.T) {
	input := "$6\r\nfoobar\r\n"
	p := resp.NewParser(bytes.NewBufferString(input))
	val, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if val.Type != resp.TypeBulkString || val.Str != "foobar" {
		t.Fatalf("expected $6 foobar, got %s", val.String())
	}
}

func TestRESPParser_NullBulkString(t *testing.T) {
	input := "$-1\r\n"
	p := resp.NewParser(bytes.NewBufferString(input))
	val, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if !val.Null {
		t.Fatalf("expected null bulk string")
	}
}

func TestRESPParser_ArrayCommand(t *testing.T) {
	// Represents: SET key value
	input := "*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n"
	p := resp.NewParser(bytes.NewBufferString(input))
	val, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if val.Type != resp.TypeArray || len(val.Array) != 3 {
		t.Fatalf("expected 3-element array, got %v", val)
	}
	if val.Array[0].Str != "SET" || val.Array[1].Str != "key" || val.Array[2].Str != "value" {
		t.Fatalf("unexpected array elements: %v", val.Array)
	}
}

func TestRESPWriter_RoundTrip(t *testing.T) {
	original := resp.MakeArray([]resp.Value{
		resp.MakeBulkString("GET"),
		resp.MakeBulkString("user:100"),
	})

	wireBytes := original.ToBytes()
	p := resp.NewParser(bytes.NewBuffer(wireBytes))
	decoded, err := p.Parse()
	if err != nil {
		t.Fatalf("failed to decode serialized bytes: %v", err)
	}

	if decoded.Type != resp.TypeArray || len(decoded.Array) != 2 {
		t.Fatalf("mismatched decoded array: %v", decoded)
	}
	if decoded.Array[0].Str != "GET" || decoded.Array[1].Str != "user:100" {
		t.Fatalf("content mismatch: %v", decoded)
	}
}
