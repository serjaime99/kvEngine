package resp

import (
	"bufio"
	"io"
	"strconv"
)

// Parser handles streaming deserialization of the Redis RESP wire protocol.
type Parser struct {
	reader *bufio.Reader
}

// NewParser creates a new RESP parser reading from any io.Reader.
func NewParser(rd io.Reader) *Parser {
	return &Parser{
		reader: bufio.NewReader(rd),
	}
}

// Parse decodes the next complete RESP Value from the underlying stream.
func (p *Parser) Parse() (Value, error) {
	prefix, err := p.reader.ReadByte()
	if err != nil {
		return Value{}, err
	}

	switch prefix {
	case TypeSimpleString:
		return p.parseSimpleString()
	case TypeError:
		return p.parseError()
	case TypeInteger:
		return p.parseInteger()
	case TypeBulkString:
		return p.parseBulkString()
	case TypeArray:
		return p.parseArray()
	default:
		return Value{}, ErrInvalidSyntax
	}
}

// readLine reads bytes until \r\n and returns the slice without the trailing \r\n.
func (p *Parser) readLine() ([]byte, error) {
	line, err := p.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	n := len(line)
	if n < 2 || line[n-2] != '\r' {
		return nil, ErrInvalidSyntax
	}
	return line[:n-2], nil
}

func (p *Parser) parseSimpleString() (Value, error) {
	line, err := p.readLine()
	if err != nil {
		return Value{}, err
	}
	return MakeSimpleString(string(line)), nil
}

func (p *Parser) parseError() (Value, error) {
	line, err := p.readLine()
	if err != nil {
		return Value{}, err
	}
	return MakeError(string(line)), nil
}

func (p *Parser) parseInteger() (Value, error) {
	line, err := p.readLine()
	if err != nil {
		return Value{}, err
	}
	num, err := strconv.ParseInt(string(line), 10, 64)
	if err != nil {
		return Value{}, ErrInvalidSyntax
	}
	return MakeInteger(num), nil
}

func (p *Parser) parseBulkString() (Value, error) {
	line, err := p.readLine()
	if err != nil {
		return Value{}, err
	}

	length, err := strconv.Atoi(string(line))
	if err != nil {
		return Value{}, ErrInvalidSyntax
	}

	// Null bulk string representation: "$-1\r\n"
	if length == -1 {
		return MakeNullBulkString(), nil
	}
	if length < -1 {
		return Value{}, ErrInvalidSyntax
	}

	// Read exact payload bytes + trailing \r\n
	buf := make([]byte, length+2)
	if _, err := io.ReadFull(p.reader, buf); err != nil {
		return Value{}, err
	}

	if buf[length] != '\r' || buf[length+1] != '\n' {
		return Value{}, ErrInvalidSyntax
	}

	return MakeBulkString(string(buf[:length])), nil
}

func (p *Parser) parseArray() (Value, error) {
	line, err := p.readLine()
	if err != nil {
		return Value{}, err
	}

	count, err := strconv.Atoi(string(line))
	if err != nil {
		return Value{}, ErrInvalidSyntax
	}

	// Null array representation: "*-1\r\n"
	if count == -1 {
		return MakeNullArray(), nil
	}
	if count < -1 {
		return Value{}, ErrInvalidSyntax
	}

	elements := make([]Value, count)
	for i := 0; i < count; i++ {
		val, err := p.Parse()
		if err != nil {
			return Value{}, err
		}
		elements[i] = val
	}

	return MakeArray(elements), nil
}
