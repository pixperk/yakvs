package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pixperk/yakvs/client"
)

func printUsage() {
	fmt.Println("\nAvailable Commands:")
	fmt.Println("  set <key> <value> <ttl-seconds>  - Set a value with TTL")
	fmt.Println("  get <key>                       - Get a value")
	fmt.Println("  delete <key>                    - Delete a value")
	fmt.Println("  ttl <key>                       - Get the TTL for a key")
	fmt.Println("  status                          - Get the Raft cluster status")
	fmt.Println("  help                            - Show this help message")
	fmt.Println("  exit                            - Exit the client")
}

func printWelcome(serverAddr string) {
	fmt.Println("┌───────────────────────────────────────────────────┐")
	fmt.Println("│                     Y A K V S                     │")
	fmt.Println("│         Yet Another Key-Value Store (Raft)        │")
	fmt.Println("├───────────────────────────────────────────────────┤")
	fmt.Printf("│ Connected to: %-35s │\n", serverAddr)
	fmt.Println("│                                                   │")
	fmt.Println("│ • Type 'help' to see available commands           │")
	fmt.Println("│ • Type 'exit' to quit                             │")
	fmt.Println("└───────────────────────────────────────────────────┘")
}

func main() {
	serverAddr := flag.String("server", "localhost:8080", "server address")
	interactive := flag.Bool("interactive", true, "run in interactive mode")
	command := flag.String("command", "", "command to run in non-interactive mode")
	flag.Parse()

	c, err := client.NewRaftClient(*serverAddr)
	if err != nil {
		fmt.Printf("Error connecting to server: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	// If command is specified, use that instead of flag.Args()
	var args []string
	if *command != "" {
		args = parseInput(*command)
		*interactive = false
	} else {
		args = flag.Args()
	}

	// Check if there are command-line arguments for non-interactive mode
	if len(args) > 0 && !*interactive {
		processCommand(c, args)
		return
	}

	// Interactive mode
	printWelcome(*serverAddr)
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\n\033[1;36myakvs-raft>\033[0m ")
		if !scanner.Scan() {
			break
		}

		input := scanner.Text()
		if input == "" {
			continue
		}

		args := parseInput(input)
		if len(args) == 0 {
			continue
		}

		if args[0] == "exit" {
			fmt.Println("Goodbye!")
			break
		}

		if args[0] == "help" {
			printUsage()
			continue
		}

		processCommand(c, args)
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading input: %v\n", err)
	}
}

// parseInput splits the input string into arguments, respecting quotes
func parseInput(input string) []string {
	var args []string
	var currentArg strings.Builder
	inQuotes := false

	for _, r := range input {
		if r == '"' {
			inQuotes = !inQuotes
			continue
		}

		if r == ' ' && !inQuotes {
			if currentArg.Len() > 0 {
				args = append(args, currentArg.String())
				currentArg.Reset()
			}
			continue
		}

		currentArg.WriteRune(r)
	}

	if currentArg.Len() > 0 {
		args = append(args, currentArg.String())
	}

	return args
}

func processCommand(c *client.RaftClient, args []string) {
	if len(args) == 0 {
		return
	}

	cmd := args[0]
	switch cmd {
	case "set":
		if len(args) < 4 {
			fmt.Println("Error: 'set' requires key, value and TTL arguments")
			fmt.Println("Usage: set <key> <value> <ttl-seconds>")
			return
		}

		key := args[1]
		value := args[2]
		ttl, err := time.ParseDuration(args[3] + "s")
		if err != nil {
			fmt.Printf("Error parsing TTL: %v\n", err)
			return
		}

		if err := c.Set(key, value, ttl); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Successfully set key '%s'\n", key)

	case "get":
		if len(args) < 2 {
			fmt.Println("Error: 'get' requires a key argument")
			fmt.Println("Usage: get <key>")
			return
		}

		key := args[1]
		value, ttl, err := c.Get(key)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Key: %s\n", key)
		fmt.Printf("Value: %s\n", value)
		fmt.Printf("TTL: %v\n", ttl)

	case "delete":
		if len(args) < 2 {
			fmt.Println("Error: 'delete' requires a key argument")
			fmt.Println("Usage: delete <key>")
			return
		}

		key := args[1]
		if err := c.Delete(key); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Successfully deleted key '%s'\n", key)

	case "ttl":
		if len(args) < 2 {
			fmt.Println("Error: 'ttl' requires a key argument")
			fmt.Println("Usage: ttl <key>")
			return
		}

		key := args[1]
		ttl, err := c.TTL(key)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("TTL for key '%s': %v\n", key, ttl)

	case "status":
		status, err := c.Status()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Cluster status: %s\n", status)

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
	}
}
