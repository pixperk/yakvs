package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pixperk/yakvs/server"
)

func main() {
	// Parse command line flags
	addr := flag.String("addr", "localhost:8080", "server address")
	logPath := flag.String("log", "kvs.log", "path to log file")
	flag.Parse()

	// Create and start server
	srv, err := server.NewServer(*addr, *logPath)
	if err != nil {
		fmt.Printf("Error creating server: %v\n", err)
		os.Exit(1)
	}

	if err := srv.Start(); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		os.Exit(1)
	}

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down server...")
	if err := srv.Stop(); err != nil {
		fmt.Printf("Error stopping server: %v\n", err)
	}
}
