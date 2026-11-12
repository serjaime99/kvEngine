package server

import (
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"

	"github.com/serjaime99/kvEngine/pkg/resp"
)

// Server coordinates the TCP network listener and client sessions.
type Server struct {
	addr       string
	dispatcher *Dispatcher
	listener   net.Listener
	stopChan   chan struct{}
	wg         sync.WaitGroup
	activeConns int64
}

// NewServer creates a new TCP server listening on the specified host:port.
func NewServer(addr string, dispatcher *Dispatcher) *Server {
	return &Server{
		addr:       addr,
		dispatcher: dispatcher,
		stopChan:   make(chan struct{}),
	}
}

// Start begins listening on the network interface and serves client connections.
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = listener

	log.Printf("[kvEngine] Server listening on %s (Redis protocol compatible)", s.addr)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				return nil // Graceful shutdown
			default:
				log.Printf("[kvEngine] Error accepting connection: %v", err)
				continue
			}
		}

		s.wg.Add(1)
		atomic.AddInt64(&s.activeConns, 1)
		go s.handleConnection(conn)
	}
}

// handleConnection services a single TCP client connection using streaming RESP.
func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		_ = conn.Close()
		atomic.AddInt64(&s.activeConns, -1)
		s.wg.Done()
	}()

	parser := resp.NewParser(conn)
	writer := resp.NewWriter(conn)

	for {
		val, err := parser.Parse()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return // Client cleanly closed connection
			}
			return // Protocol error or socket disconnect
		}

		if val.Type != resp.TypeArray {
			_ = writer.WriteValue(resp.MakeError("ERR protocol syntax error: expected command array"))
			_ = writer.Flush()
			continue
		}

		args := make([]string, len(val.Array))
		for i, item := range val.Array {
			args[i] = item.Str
		}

		// Execute against storage engine
		response := s.dispatcher.Execute(args)

		if err := writer.WriteValue(response); err != nil {
			return
		}
		if err := writer.Flush(); err != nil {
			return
		}
	}
}

// Stop initiates a clean graceful shutdown.
func (s *Server) Stop() error {
	close(s.stopChan)
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.wg.Wait()
	return nil
}

// ActiveConnections returns the number of currently attached clients.
func (s *Server) ActiveConnections() int64 {
	return atomic.LoadInt64(&s.activeConns)
}
