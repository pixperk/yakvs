package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/pixperk/yakvs/store"
)

type Server struct {
	store     *store.Store
	addr      string
	listener  net.Listener
	isRunning bool
}

type Command struct {
	Op        string        `json:"op"`
	Key       string        `json:"key"`
	Value     string        `json:"value,omitempty"`
	ExpiresIn time.Duration `json:"expires_in,omitempty"`
}

type Response struct {
	Status  string        `json:"status"`
	Message string        `json:"message,omitempty"`
	Value   string        `json:"value,omitempty"`
	TTL     time.Duration `json:"ttl,omitempty"`
}

func NewServer(addr string, logFilePath string) (*Server, error) {
	s, err := store.NewStore(logFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	return &Server{
		store: s,
		addr:  addr,
	}, nil
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}

	s.listener = listener
	s.isRunning = true
	fmt.Printf("Server started on %s\n", s.addr)

	s.store.StartBackgroundCleaner()

	go s.acceptConnections()

	return nil
}

func (s *Server) Stop() error {
	if !s.isRunning {
		return nil
	}

	s.isRunning = false
	return s.listener.Close()
}

func (s *Server) acceptConnections() {
	for s.isRunning {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.isRunning {
				fmt.Printf("Error accepting connection: %v\n", err)
			}
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		cmdText := scanner.Text()
		if cmdText == "" {
			continue
		}

		var cmd Command
		if err := json.Unmarshal([]byte(cmdText), &cmd); err != nil {
			sendResponse(conn, Response{
				Status:  "error",
				Message: "Invalid command format",
			})
			continue
		}

		resp := s.processCommand(cmd)
		sendResponse(conn, resp)
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading from connection: %v\n", err)
	}
}

func (s *Server) processCommand(cmd Command) Response {
	switch strings.ToUpper(cmd.Op) {
	case "SET":
		if cmd.Key == "" {
			return Response{Status: "error", Message: "Key is required"}
		}

		value := store.NewValue(cmd.Value, cmd.ExpiresIn)
		s.store.Set(cmd.Key, value)
		return Response{Status: "success"}

	case "GET":
		if cmd.Key == "" {
			return Response{Status: "error", Message: "Key is required"}
		}

		value, exists := s.store.Get(cmd.Key)
		if !exists {
			return Response{Status: "error", Message: "Key not found"}
		}

		// Get TTL
		ttl, _ := s.store.TTL(cmd.Key)

		return Response{Status: "success", Value: value.Data, TTL: ttl}

	case "DELETE":
		if cmd.Key == "" {
			return Response{Status: "error", Message: "Key is required"}
		}

		s.store.Delete(cmd.Key)
		return Response{Status: "success"}

	case "TTL":
		if cmd.Key == "" {
			return Response{Status: "error", Message: "Key is required"}
		}

		ttl, exists := s.store.TTL(cmd.Key)
		if !exists {
			return Response{Status: "error", Message: "Key not found or expired"}
		}

		return Response{Status: "success", TTL: ttl}

	default:
		return Response{Status: "error", Message: "Unknown command"}
	}
}

func sendResponse(conn net.Conn, resp Response) {
	jsonResp, err := json.Marshal(resp)
	if err != nil {
		fmt.Printf("Error marshaling response: %v\n", err)
		return
	}

	jsonResp = append(jsonResp, '\n')
	if _, err := conn.Write(jsonResp); err != nil {
		fmt.Printf("Error sending response: %v\n", err)
	}
}
