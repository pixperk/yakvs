package raft

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/pixperk/yakvs/store"
)

// Raft-backed key-value store
type RaftStore struct {
	store       *store.Store
	raft        *raft.Raft
	fsm         *FSM
	transport   *raft.NetworkTransport
	logStore    *raftboltdb.BoltStore
	stableStore *raftboltdb.BoltStore
	snapshots   *raft.FileSnapshotStore
	raftDir     string
	nodeID      string
	addr        string
	bootstrap   bool
}

type Config struct {
	NodeID      string
	RaftDir     string
	RaftAddr    string
	Bootstrap   bool
	LogFilePath string
}

func NewRaftStore(config Config) (*RaftStore, error) {
	// Create the underlying store
	s, err := store.NewStore(config.LogFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	fsm := NewFSM(s)

	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(config.NodeID)

	//Raft transport
	addr, err := net.ResolveTCPAddr("tcp", config.RaftAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve TCP address: %w", err)
	}
	transport, err := raft.NewTCPTransport(config.RaftAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create TCP transport: %w", err)
	}

	// Create the log store and stable store
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(config.RaftDir, "raft-log.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create bolt store for logs: %w", err)
	}
	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(config.RaftDir, "raft-stable.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create bolt store for stable storage: %w", err)
	}

	// Create the snapshot store
	snapshots, err := raft.NewFileSnapshotStore(config.RaftDir, 3, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create file snapshot store: %w", err)
	}

	// Create the Raft instance
	r, err := raft.NewRaft(raftConfig, fsm, logStore, stableStore, snapshots, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create new raft: %w", err)
	}

	rs := &RaftStore{
		store:       s,
		raft:        r,
		fsm:         fsm,
		transport:   transport,
		logStore:    logStore,
		stableStore: stableStore,
		snapshots:   snapshots,
		raftDir:     config.RaftDir,
		nodeID:      config.NodeID,
		addr:        config.RaftAddr,
		bootstrap:   config.Bootstrap,
	}

	// Bootstrap the cluster if needed
	if config.Bootstrap {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raft.ServerID(config.NodeID),
					Address: transport.LocalAddr(),
				},
			},
		}
		r.BootstrapCluster(configuration)
	}

	return rs, nil
}

func (rs *RaftStore) Get(key string) (store.Value, bool) {
	return rs.store.Get(key)
}

func (rs *RaftStore) Set(key string, value store.Value) error {
	if rs.raft.State() != raft.Leader {
		return fmt.Errorf("not the leader")
	}

	cmd := Command{
		Op:        "SET",
		Key:       key,
		Value:     value.Data,
		ExpiresAt: value.ExpiresAt,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	future := rs.raft.Apply(data, 500*time.Millisecond)
	return future.Error()
}

func (rs *RaftStore) Delete(key string) error {
	if rs.raft.State() != raft.Leader {
		return fmt.Errorf("not the leader")
	}

	cmd := Command{
		Op:  "DELETE",
		Key: key,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	future := rs.raft.Apply(data, 500*time.Millisecond)
	return future.Error()
}

func (rs *RaftStore) TTL(key string) (time.Duration, bool) {
	return rs.store.TTL(key)
}

func (rs *RaftStore) IsLeader() bool {
	return rs.raft.State() == raft.Leader
}

func (rs *RaftStore) GetLeader() string {
	addr := rs.raft.Leader()
	if addr == "" {
		return ""
	}
	return string(addr)
}

// Join adds a node to the cluster
func (rs *RaftStore) Join(nodeID, addr string) error {
	if !rs.IsLeader() {
		return fmt.Errorf("not the leader")
	}

	configFuture := rs.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return err
	}

	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(nodeID) || srv.Address == raft.ServerAddress(addr) {
			// Already joined
			return nil
		}
	}

	future := rs.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, 0)
	if err := future.Error(); err != nil {
		return err
	}

	return nil
}

// Leave removes this node from the cluster
func (rs *RaftStore) Leave() error {
	if rs.IsLeader() {

		return rs.raft.LeadershipTransfer().Error()
	}
	return nil
}

// Shutdown closes the Raft cluster
func (rs *RaftStore) Shutdown() error {
	// Shutdown the Raft instance
	future := rs.raft.Shutdown()
	if err := future.Error(); err != nil {
		return err
	}

	// Close the stores
	if err := rs.logStore.Close(); err != nil {
		return err
	}
	if err := rs.stableStore.Close(); err != nil {
		return err
	}

	return nil
}

func (rs *RaftStore) BackgroundCleaner() {
	rs.store.BackgroundCleaner()
}

func (rs *RaftStore) StartBackgroundCleaner() {
	rs.store.StartBackgroundCleaner()
}

// TakeSnapshot forces the creation of a snapshot
func (rs *RaftStore) TakeSnapshot() error {
	if rs.raft.State() != raft.Leader {
		return fmt.Errorf("not the leader")
	}

	future := rs.raft.Snapshot()
	return future.Error()
}
