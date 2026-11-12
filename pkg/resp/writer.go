package resp

import (
	"bufio"
	"io"
)

// Writer serializes RESP Values into a buffered TCP stream.
type Writer struct {
	writer *bufio.Writer
}

// NewWriter wraps an io.Writer with buffered RESP serialization.
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		writer: bufio.NewWriter(w),
	}
}

// WriteValue serializes and writes a Value to the underlying stream.
func (w *Writer) WriteValue(v Value) error {
	bytes := v.ToBytes()
	_, err := w.writer.Write(bytes)
	return err
}

// Flush flushes buffered bytes down to the TCP socket.
func (w *Writer) Flush() error {
	return w.writer.Flush()
}
