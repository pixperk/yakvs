package raft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func JoinCluster(leaderAPI, nodeID, raftAddr string) error {
	joinURL := fmt.Sprintf("http://%s/join", leaderAPI)

	req := JoinRequest{
		NodeID: nodeID,
		Addr:   raftAddr,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal join request: %w", err)
	}

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Post(joinURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send join request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("join request failed with status: %s", resp.Status)
	}

	return nil
}
