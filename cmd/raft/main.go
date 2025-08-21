package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pixperk/yakvs/raft"
	"github.com/pixperk/yakvs/server"
)

func main() {
	// Parse command line flags
	nodeID := flag.String("id", "", "unique node ID")
	raftAddr := flag.String("raft", "localhost:7000", "raft transport address")
	tcpAddr := flag.String("tcp", "localhost:8080", "TCP server address")
	apiAddr := flag.String("api", "localhost:8081", "HTTP API address")
	raftDir := flag.String("dir", "raft-data", "directory for Raft data")
	joinAddr := flag.String("join", "", "leader address to join (empty for first node)")
	bootstrap := flag.Bool("bootstrap", false, "bootstrap the cluster with this node")

	flag.Parse()

	// Check required parameters
	if *nodeID == "" {
		log.Fatal("Error: node ID is required")
	}

	// Create data directory
	dataDir := filepath.Join(*raftDir, *nodeID)
	os.MkdirAll(dataDir, 0755)

	logFilePath := filepath.Join(dataDir, "kvs.log")

	// Create and start RaftStore
	config := raft.Config{
		NodeID:      *nodeID,
		RaftDir:     dataDir,
		RaftAddr:    *raftAddr,
		Bootstrap:   *bootstrap,
		LogFilePath: logFilePath,
	}

	raftStore, err := raft.NewRaftStore(config)
	if err != nil {
		log.Fatalf("Failed to create Raft store: %v", err)
	}

	// Create and start API server
	api := raft.NewAPI(raftStore, *apiAddr)
	if err := api.Start(); err != nil {
		log.Fatalf("Failed to start API server: %v", err)
	}

	// Create and start TCP server
	srv := server.NewRaftServer(*tcpAddr, raftStore)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start TCP server: %v", err)
	}

	// Join an existing cluster if specified
	if *joinAddr != "" && *joinAddr != *apiAddr {
		fmt.Printf("Joining cluster at %s\n", *joinAddr)

		// Create HTTP client to join the cluster
		joinURL := fmt.Sprintf("http://%s/join", *joinAddr)
		payload := fmt.Sprintf(`{"node_id":"%s","addr":"%s"}`, *nodeID, *raftAddr)

		// In a real implementation, you would make an HTTP POST request here
		// For simplicity, we'll just print the command
		fmt.Printf("curl -X POST -d '%s' %s\n", payload, joinURL)
	}

	fmt.Printf("Raft node %s started\n", *nodeID)
	fmt.Printf("- Raft Address: %s\n", *raftAddr)
	fmt.Printf("- TCP Address:  %s\n", *tcpAddr)
	fmt.Printf("- API Address:  %s\n", *apiAddr)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down...")

	// Graceful shutdown
	srv.Stop()
	api.Stop()
	raftStore.Shutdown()
}
