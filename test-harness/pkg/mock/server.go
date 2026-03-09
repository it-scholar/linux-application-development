// Package mock provides mock protocol server capabilities
package mock

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// Server represents a mock protocol server
type Server struct {
	config   Config
	listener net.Listener
	handlers map[uint8]Handler
	sessions map[string]*Session
	mu       sync.RWMutex
	started  bool
	stopCh   chan struct{}
}

// Config holds server configuration
type Config struct {
	Address    string
	Port       int
	Protocol   string // tcp, udp
	MaxClients int
	Timeout    time.Duration
}

// Handler handles incoming messages
type Handler func(msg *Message) (*Message, error)

// Session represents a client session
type Session struct {
	ID       string
	Conn     net.Conn
	Started  time.Time
	LastSeen time.Time
	mu       sync.Mutex
}

// Message represents a protocol message
type Message struct {
	Type      uint8
	Sequence  uint32
	Payload   []byte
	Timestamp time.Time
}

// NewServer creates a new mock server
func NewServer(config Config) *Server {
	if config.Address == "" {
		config.Address = "localhost"
	}
	if config.Port == 0 {
		config.Port = 9000
	}
	if config.Protocol == "" {
		config.Protocol = "tcp"
	}
	if config.MaxClients == 0 {
		config.MaxClients = 100
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &Server{
		config:   config,
		handlers: make(map[uint8]Handler),
		sessions: make(map[string]*Session),
		stopCh:   make(chan struct{}),
	}
}

// RegisterHandler registers a handler for a message type
func (s *Server) RegisterHandler(msgType uint8, handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[msgType] = handler
}

// Start starts the server
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("server already started")
	}

	address := fmt.Sprintf("%s:%d", s.config.Address, s.config.Port)
	listener, err := net.Listen(s.config.Protocol, address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.listener = listener
	s.started = true

	// Start accepting connections
	go s.acceptLoop()

	return nil
}

// Stop stops the server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	close(s.stopCh)
	s.started = false

	// Close all sessions
	for _, session := range s.sessions {
		session.Conn.Close()
	}
	s.sessions = make(map[string]*Session)

	return s.listener.Close()
}

// IsRunning returns true if server is running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started
}

// Address returns the server address
func (s *Server) Address() string {
	return fmt.Sprintf("%s:%d", s.config.Address, s.config.Port)
}

func (s *Server) acceptLoop() {
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				continue
			}
		}

		// Check max clients
		s.mu.RLock()
		clientCount := len(s.sessions)
		s.mu.RUnlock()

		if clientCount >= s.config.MaxClients {
			conn.Close()
			continue
		}

		// Handle connection
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	// Create session
	session := &Session{
		ID:       generateSessionID(),
		Conn:     conn,
		Started:  time.Now(),
		LastSeen: time.Now(),
	}

	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.sessions, session.ID)
		s.mu.Unlock()
		conn.Close()
	}()

	// Handle messages
	buffer := make([]byte, 4096)
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		// Set read timeout
		conn.SetReadDeadline(time.Now().Add(s.config.Timeout))

		n, err := conn.Read(buffer)
		if err != nil {
			return
		}

		// Update last seen
		session.mu.Lock()
		session.LastSeen = time.Now()
		session.mu.Unlock()

		// Parse and handle message
		if n > 0 {
			s.handleMessage(session, buffer[:n])
		}
	}
}

func (s *Server) handleMessage(session *Session, data []byte) {
	// Parse message (simplified)
	if len(data) < 24 {
		return
	}

	msg := &Message{
		Type:      data[6], // Simplified parsing
		Sequence:  uint32(data[10])<<24 | uint32(data[11])<<16 | uint32(data[12])<<8 | uint32(data[13]),
		Payload:   data[24:],
		Timestamp: time.Now(),
	}

	// Find handler
	s.mu.RLock()
	handler, ok := s.handlers[msg.Type]
	s.mu.RUnlock()

	if !ok {
		// Send error response
		s.sendError(session, msg.Sequence, "unknown message type")
		return
	}

	// Call handler
	response, err := handler(msg)
	if err != nil {
		s.sendError(session, msg.Sequence, err.Error())
		return
	}

	// Send response
	if response != nil {
		s.sendMessage(session, response)
	}
}

func (s *Server) sendMessage(session *Session, msg *Message) {
	// Build message bytes (simplified)
	data := make([]byte, 24+len(msg.Payload))
	// ... encode header ...
	copy(data[24:], msg.Payload)

	session.Conn.Write(data)
}

func (s *Server) sendError(session *Session, sequence uint32, errorMsg string) {
	response := &Message{
		Type:     0x04, // Error message type
		Sequence: sequence,
		Payload:  []byte(errorMsg),
	}
	s.sendMessage(session, response)
}

// GetSessionCount returns the number of active sessions
func (s *Server) GetSessionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// GetSessions returns all active sessions
func (s *Server) GetSessions() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

func generateSessionID() string {
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}

// DefaultHandlers returns default message handlers
func DefaultHandlers() map[uint8]Handler {
	return map[uint8]Handler{
		0x01: func(msg *Message) (*Message, error) {
			// Heartbeat handler
			return &Message{
				Type:     0x01,
				Sequence: msg.Sequence,
				Payload:  []byte("pong"),
			}, nil
		},
		0x02: func(msg *Message) (*Message, error) {
			// Data request handler
			return &Message{
				Type:     0x03,
				Sequence: msg.Sequence,
				Payload:  []byte("data response"),
			}, nil
		},
		0x05: func(msg *Message) (*Message, error) {
			// Query handler
			return &Message{
				Type:     0x06,
				Sequence: msg.Sequence,
				Payload:  []byte("query result"),
			}, nil
		},
	}
}

// CreateMockServer creates a mock server with default handlers
func CreateMockServer(port int) *Server {
	server := NewServer(Config{
		Address: "localhost",
		Port:    port,
	})

	// Register default handlers
	for msgType, handler := range DefaultHandlers() {
		server.RegisterHandler(msgType, handler)
	}

	return server
}
