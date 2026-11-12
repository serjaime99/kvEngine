package resp

import (
	"errors"
	"fmt"
	"strconv"
)

// RESP Protocol byte prefixes
const (
	TypeSimpleString = '+'
	TypeError        = '-'
	TypeInteger      = ':'
	TypeBulkString   = '$'
	TypeArray        = '*'
)

var (
	ErrInvalidSyntax = errors.New("resp: invalid protocol syntax")
	ErrUnexpectedEOF = errors.New("resp: unexpected end of stream")
)

// Value represents a typed RESP primitive or composite element.
type Value struct {
	Type  byte     // Type marker: '+', '-', ':', '$', '*'
	Str   string   // Used for SimpleString, Error, and BulkString
	Num   int64    // Used for Integer
	Array []Value  // Used for Array
	Null  bool     // True if BulkString or Array represents a NULL value ($-1 or *-1)
}

// String returns a human-readable representation for debugging.
func (v Value) String() string {
	switch v.Type {
	case TypeSimpleString:
		return fmt.Sprintf("+%s", v.Str)
	case TypeError:
		return fmt.Sprintf("-%s", v.Str)
	case TypeInteger:
		return fmt.Sprintf(":%d", v.Num)
	case TypeBulkString:
		if v.Null {
			return "$-1"
		}
		return fmt.Sprintf("$%d\r\n%s", len(v.Str), v.Str)
	case TypeArray:
		if v.Null {
			return "*-1"
		}
		return fmt.Sprintf("*%d elements", len(v.Array))
	default:
		return "unknown"
	}
}

// Helper constructors for idiomatic value generation
func MakeSimpleString(s string) Value {
	return Value{Type: TypeSimpleString, Str: s}
}

func MakeError(msg string) Value {
	return Value{Type: TypeError, Str: msg}
}

func MakeInteger(n int64) Value {
	return Value{Type: TypeInteger, Num: n}
}

func MakeBulkString(s string) Value {
	return Value{Type: TypeBulkString, Str: s}
}

func MakeNullBulkString() Value {
	return Value{Type: TypeBulkString, Null: true}
}

func MakeArray(items []Value) Value {
	return Value{Type: TypeArray, Array: items}
}

func MakeNullArray() Value {
	return Value{Type: TypeArray, Null: true}
}

// ToBytes serializes any Value directly into valid RESP wire bytes.
func (v Value) ToBytes() []byte {
	switch v.Type {
	case TypeSimpleString:
		return []byte("+" + v.Str + "\r\n")
	case TypeError:
		return []byte("-" + v.Str + "\r\n")
	case TypeInteger:
		return []byte(":" + strconv.FormatInt(v.Num, 10) + "\r\n")
	case TypeBulkString:
		if v.Null {
			return []byte("$-1\r\n")
		}
		return []byte("$" + strconv.Itoa(len(v.Str)) + "\r\n" + v.Str + "\r\n")
	case TypeArray:
		if v.Null {
			return []byte("*-1\r\n")
		}
		var buf []byte
		buf = append(buf, []byte("*"+strconv.Itoa(len(v.Array))+"\r\n")...)
		for _, item := range v.Array {
			buf = append(buf, item.ToBytes()...)
		}
		return buf
	default:
		return []byte("-ERR unknown type\r\n")
	}
}
