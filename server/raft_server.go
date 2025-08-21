package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/pixperk/yakvs/raft"
	"github.com/pixperk/yakvs/store"
)

type RaftServer struct {
	store     *raft.RaftStore
	addr      string
	listener  net.Listener
	isRunning bool
}

func NewRaftServer(addr string, store *raft.RaftStore) *RaftServer {
	return &RaftServer{
		store: store,
		addr:  addr,
	}
}

func (s *RaftServer) Start() error {
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

func (s *RaftServer) Stop() error {
	if !s.isRunning {
		return nil
	}

	s.isRunning = false
	return s.listener.Close()
}

func (s *RaftServer) acceptConnections() {
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

func (s *RaftServer) handleConnection(conn net.Conn) {
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

func (s *RaftServer) processCommand(cmd Command) Response {
	switch strings.ToUpper(cmd.Op) {
	case "SET":
		if cmd.Key == "" {
			return Response{Status: "error", Message: "Key is required"}
		}

		// Create value
		value := store.Value{
			Data:      cmd.Value,
			ExpiresAt: time.Now().Add(cmd.ExpiresIn),
		}

		err := s.store.Set(cmd.Key, value)
		if err != nil {
			// If not the leader, inform client
			if strings.Contains(err.Error(), "not the leader") {
				leaderAddr := s.store.GetLeader()
				return Response{
					Status:  "redirect",
					Message: fmt.Sprintf("Not the leader, try: %s", leaderAddr),
				}
			}
			return Response{Status: "error", Message: err.Error()}
		}

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

		err := s.store.Delete(cmd.Key)
		if err != nil {
			// If not the leader, inform client
			if strings.Contains(err.Error(), "not the leader") {
				leaderAddr := s.store.GetLeader()
				return Response{
					Status:  "redirect",
					Message: fmt.Sprintf("Not the leader, try: %s", leaderAddr),
				}
			}
			return Response{Status: "error", Message: err.Error()}
		}

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

	case "STATUS":
		isLeader := s.store.IsLeader()
		status := "follower"
		if isLeader {
			status = "leader"
		}

		return Response{
			Status:  "success",
			Message: fmt.Sprintf("Node status: %s", status),
		}

	default:
		return Response{Status: "error", Message: "Unknown command"}
	}
}
