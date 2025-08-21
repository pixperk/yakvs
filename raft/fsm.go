package raft

import (
	"encoding/json"
	"io"
	"time"

	"github.com/hashicorp/raft"
	"github.com/pixperk/yakvs/store"
)

type Command struct {
	Op        string    `json:"op"`
	Key       string    `json:"key"`
	Value     string    `json:"value,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

type FSM struct {
	store *store.Store
}

func NewFSM(store *store.Store) *FSM {
	return &FSM{
		store: store,
	}
}

// Apply applies a Raft log entry to the store
func (f *FSM) Apply(log *raft.Log) interface{} {
	var cmd Command
	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		return err
	}

	switch cmd.Op {
	case "SET":
		value := store.Value{
			Data:      cmd.Value,
			ExpiresAt: cmd.ExpiresAt,
		}
		f.store.Set(cmd.Key, value)
		return nil
	case "DELETE":
		f.store.Delete(cmd.Key)
		return nil
	default:
		return nil
	}
}

// Snapshot returns a snapshot of the store
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	data := make(map[string]store.Value)

	f.store.Range(func(key string, value store.Value) bool {
		data[key] = value
		return true
	})

	return &Snapshot{data: data}, nil
}

func (f *FSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()

	decoder := json.NewDecoder(rc)

	var data map[string]store.Value
	if err := decoder.Decode(&data); err != nil {
		return err
	}

	// Clear the current store
	f.store.Clear()

	// Restore all key-value pairs from snapshot
	for key, value := range data {
		f.store.Set(key, value)
	}

	return nil
}

// Snapshot implements the raft.FSMSnapshot interface
type Snapshot struct {
	data map[string]store.Value
}

func (s *Snapshot) Persist(sink raft.SnapshotSink) error {
	defer sink.Close()

	encoder := json.NewEncoder(sink)
	if err := encoder.Encode(s.data); err != nil {
		sink.Cancel()
		return err
	}

	return nil
}

func (s *Snapshot) Release() {
	// Release resources if needed
	s.data = nil
}
