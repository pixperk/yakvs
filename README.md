# YAKVS - Yet Another Key-Value Store

A high-performance, distributed key-value store in Go with support for standalone and clustered deployments using the Raft consensus protocol.

## Features

- **Persistence**: All operations are logged to disk and recovered on restart
- **Expiry**: Keys can have optional TTL (Time-To-Live)
- **Distribution**: Raft consensus protocol ensures data consistency across a cluster
- **High availability**: Automatic failover in case of node failures
- **Simple client interface**: Easy-to-use client for both standalone and clustered modes

## Table of Contents

- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
- [Usage](#usage)
  - [Running a Standalone Server](#running-a-standalone-server)
  - [Running a Clustered Server](#running-a-clustered-server)
  - [Using the Client](#using-the-client)
- [Implementation Details](#implementation-details)
  - [Project Structure](#project-structure)
  - [Standalone Mode](#standalone-mode)
  - [Clustered Mode](#clustered-mode)
  - [Client](#client)
  - [Data Persistence](#data-persistence)
- [API Reference](#api-reference)
- [Development](#development)
  - [Building from Source](#building-from-source)
  - [Running Tests](#running-tests)

## Getting Started

### Prerequisites

- Go 1.21 or higher

### Installation

```bash
# Clone the repository
git clone https://github.com/pixperk/yakvs.git
cd yakvs

# Build the binaries
go build -o kvs-server ./cmd/server
go build -o kvs-client ./cmd/client
go build -o raft-server ./cmd/raft
go build -o raft-client ./cmd/raft-client
```

## Usage

### Running a Standalone Server

For simple non-distributed usage, you can run YAKVS in standalone mode:

```bash
# Start a standalone server on default port (localhost:8080)
./kvs-server

# Start with custom address and log path
./kvs-server -addr localhost:9090 -log custom_path.log
```

### Running a Clustered Server

For high availability and fault tolerance, you can run YAKVS in clustered mode using Raft:

```bash
# Start the first node (bootstrap node)
./raft-server -id node1 -raft localhost:7000 -tcp localhost:8080 -api localhost:8081 -bootstrap

# Start the second node and join the cluster
./raft-server -id node2 -raft localhost:7001 -tcp localhost:8082 -api localhost:8083 -join localhost:8081

# Start the third node and join the cluster
./raft-server -id node3 -raft localhost:7002 -tcp localhost:8084 -api localhost:8085 -join localhost:8081
```

The parameters are:
- `-id`: Unique identifier for the node
- `-raft`: Raft consensus protocol address
- `-tcp`: TCP server address for client connections
- `-api`: HTTP API address for administrative operations
- `-dir`: Directory for Raft data (default: "raft-data")
- `-bootstrap`: Flag to bootstrap a new cluster with this node
- `-join`: Address of an existing node to join the cluster

### Using the Client

#### Standalone Mode Client

```bash
# Start the client and connect to server
./kvs-client -addr localhost:8080
```

#### Clustered Mode Client

```bash
# Start the client and connect to any node in the cluster
./raft-client -addr localhost:8080
```

#### Client Commands

Once connected, you can interact with the key-value store using these commands:

```
SET <key> <value> [expiry_in_seconds]  # Store a key with optional expiry time
GET <key>                              # Retrieve a value
DELETE <key>                           # Remove a key
QUIT                                   # Exit the client
```

Example:
```
SET mykey "Hello World" 300  # Set with 5-minute expiry
GET mykey                    # Retrieve the value
DELETE mykey                 # Delete the key
```

## Implementation Details

### Project Structure

```
├── client/               # Client implementation
│   ├── client.go         # Standalone client
│   └── raft_client.go    # Raft client
├── cmd/                  # Command-line tools
│   ├── client/           # Standalone client command
│   ├── raft/             # Raft server command
│   ├── raft-client/      # Raft client command
│   └── server/           # Standalone server command
├── raft/                 # Raft implementation
│   ├── api.go            # HTTP API for Raft operations
│   ├── fsm.go            # Finite State Machine for Raft
│   ├── join.go           # Node join operations
│   └── raft_store.go     # Raft-backed store
├── raft-data/            # Raft data directory
├── server/               # Server implementation
│   ├── raft_server.go    # Raft server wrapper
│   └── server.go         # Standalone server
└── store/                # Core store implementation
    └── store.go          # Key-value store with persistence
```

### Standalone Mode

In standalone mode, YAKVS runs as a simple TCP server that processes commands directly against the local store. All operations are logged to disk for persistence.

Key components:
- `store.Store`: The in-memory key-value store with disk persistence
- `server.Server`: TCP server that handles client connections

### Clustered Mode

In clustered mode, YAKVS uses the Raft consensus protocol to maintain consistency across multiple nodes. Write operations are forwarded to the leader, while reads can be served by any node.

Key components:
- `raft.RaftStore`: Raft-backed distributed store
- `raft.FSM`: Finite State Machine that applies operations to the store
- `server.RaftServer`: TCP server that forwards client commands to the Raft cluster

### Client

The client provides a simple interface for interacting with both standalone and clustered servers:

- `client.Client`: Basic client for standalone mode
- `client.RaftClient`: Enhanced client for clustered mode

### Data Persistence

Data persistence is achieved through two mechanisms:

1. **Command logging**: Each write operation (SET/DELETE) is logged to a text file in an append-only format
2. **Raft persistence**: In clustered mode, Raft logs and snapshots provide additional durability

On restart, the store is rebuilt by replaying the command log.

## API Reference

### Store Operations

```go
// Set a value with expiry
Set(key, value string, expiresIn time.Duration) error

// Get a value and its remaining TTL
Get(key string) (value string, ttl time.Duration, error)

// Delete a value
Delete(key string) error
```

### Raft Operations

```go
// Join a node to the cluster
JoinNode(nodeID, addr string) error

// Leave the cluster
LeaveCluster() error

// Get cluster state information
GetClusterState() (ClusterState, error)
```

## Development

### Building from Source

```bash
# Build all components
go build ./...

# Build specific components
go build -o kvs-server ./cmd/server
go build -o kvs-client ./cmd/client
go build -o raft-server ./cmd/raft
go build -o raft-client ./cmd/raft-client
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./store
go test ./raft
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [HashiCorp Raft](https://github.com/hashicorp/raft) - Consensus protocol implementation
- [BoltDB](https://github.com/boltdb/bolt) - Key-value store used for Raft storage
