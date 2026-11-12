package ipc

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	// HandshakeToken ensures communication is legitimately for OmniFloat.
	HandshakeToken = "OMNIFLOAT_IPC_v1"
)

// PreferredPorts are ports tried in sequence to avoid Windows excluded/Hyper-V ranges.
var PreferredPorts = []int{28923, 38923, 18923}

var (
	ErrAlreadyRunning = errors.New("omnifloat is already running")
)

// Server listens for incoming IPC commands from CLI or other instances.
type Server struct {
	listener net.Listener
	handler  func(command string)
	quit     chan struct{}
	mu       sync.Mutex
	closed   bool
	port     int
}

// TrySendToggle attempts to connect to an existing running OmniFloat instance
// and sends the TOGGLE command. Returns true if successful.
func TrySendToggle() bool {
	return SendCommand("TOGGLE")
}

// SendCommand sends an arbitrary command (e.g. TOGGLE, SHOW, HIDE) to a running instance.
func SendCommand(command string) bool {
	for _, port := range PreferredPorts {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		conn, err := net.DialTimeout("tcp", addr, 150*time.Millisecond)
		if err != nil {
			continue
		}

		_ = conn.SetDeadline(time.Now().Add(400 * time.Millisecond))
		payload := fmt.Sprintf("%s:%s\n", HandshakeToken, strings.TrimSpace(command))
		if _, err := conn.Write([]byte(payload)); err != nil {
			conn.Close()
			continue
		}

		reader := bufio.NewReader(conn)
		resp, err := reader.ReadString('\n')
		conn.Close()
		if err != nil {
			continue
		}

		if strings.TrimSpace(resp) == "OK" {
			return true
		}
	}
	return false
}

// StartServer binds to the first available local IPC port and listens for incoming commands.
func StartServer(handler func(command string)) (*Server, error) {
	var listener net.Listener
	var boundPort int
	var lastErr error

	for _, port := range PreferredPorts {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		l, err := net.Listen("tcp", addr)
		if err == nil {
			listener = l
			boundPort = port
			break
		}
		lastErr = err
	}

	if listener == nil {
		return nil, fmt.Errorf("unable to bind any IPC port: %w", lastErr)
	}

	s := &Server{
		listener: listener,
		handler:  handler,
		quit:     make(chan struct{}),
		port:     boundPort,
	}

	go s.listenLoop()
	return s, nil
}

func (s *Server) listenLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				// Listener error or closed
				return
			}
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(1 * time.Second))

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	line = strings.TrimSpace(line)
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 || parts[0] != HandshakeToken {
		_, _ = conn.Write([]byte("ERR:INVALID_TOKEN\n"))
		return
	}

	command := parts[1]
	if s.handler != nil {
		// Execute command in a goroutine so it doesn't block IPC response
		go s.handler(command)
	}

	_, _ = conn.Write([]byte("OK\n"))
}

// Close terminates the IPC server.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.quit)
	return s.listener.Close()
}
