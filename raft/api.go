package raft

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type API struct {
	store     *RaftStore
	apiAddr   string
	apiServer *http.Server
	mu        sync.Mutex
}

type JoinRequest struct {
	NodeID string `json:"node_id"`
	Addr   string `json:"addr"`
}

func NewAPI(store *RaftStore, apiAddr string) *API {
	return &API{
		store:   store,
		apiAddr: apiAddr,
	}
}

func (a *API) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/join", a.handleJoin)
	mux.HandleFunc("/status", a.handleStatus)
	mux.HandleFunc("/snapshot", a.handleSnapshot)

	a.apiServer = &http.Server{
		Addr:    a.apiAddr,
		Handler: mux,
	}

	go func() {
		if err := a.apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Error starting API server: %v\n", err)
		}
	}()

	return nil
}

func (a *API) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.apiServer != nil {
		return a.apiServer.Close()
	}
	return nil
}

// handleJoin handles requests to join the cluster
func (a *API) handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := a.store.Join(req.NodeID, req.Addr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// StatusResponse represents the status of the Raft cluster
type StatusResponse struct {
	NodeID  string `json:"node_id"`
	Addr    string `json:"addr"`
	Leader  bool   `json:"leader"`
	Leading string `json:"leading,omitempty"`
}

// handleStatus handles requests for the cluster status
func (a *API) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := StatusResponse{
		NodeID: a.store.nodeID,
		Addr:   a.store.addr,
		Leader: a.store.IsLeader(),
	}

	if !resp.Leader {
		resp.Leading = a.store.GetLeader()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleSnapshot handles requests to create a snapshot
func (a *API) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !a.store.IsLeader() {
		leaderAddr := a.store.GetLeader()
		http.Error(w, "Not the leader, try: "+leaderAddr, http.StatusBadRequest)
		return
	}

	err := a.store.TakeSnapshot()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Snapshot created successfully"))
}
