package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

type RaftClient struct {
	conn       net.Conn
	reader     *bufio.Reader
	serverAddr string
	maxRetries int
	retryDelay time.Duration
}

func NewRaftClient(serverAddr string) (*RaftClient, error) {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server at %s: %w", serverAddr, err)
	}

	return &RaftClient{
		conn:       conn,
		reader:     bufio.NewReader(conn),
		serverAddr: serverAddr,
		maxRetries: 3,
		retryDelay: 500 * time.Millisecond,
	}, nil
}

func (c *RaftClient) Close() error {
	return c.conn.Close()
}

func (c *RaftClient) Set(key, value string, expiresIn time.Duration) error {
	cmd := Command{
		Op:        "SET",
		Key:       key,
		Value:     value,
		ExpiresIn: expiresIn,
	}

	for retry := 0; retry <= c.maxRetries; retry++ {
		resp, err := c.sendCommand(cmd)
		if err != nil {
			return err
		}

		if resp.Status == "success" {
			return nil
		} else if resp.Status == "redirect" {
			newAddr := extractServerAddress(resp.Message)
			if newAddr != "" && newAddr != c.serverAddr {
				if err := c.reconnectToServer(newAddr); err != nil {
					return err
				}
				continue
			}
		}

		return fmt.Errorf("server error: %s", resp.Message)
	}

	return fmt.Errorf("max retries reached")
}

func (c *RaftClient) Get(key string) (string, time.Duration, error) {
	cmd := Command{
		Op:  "GET",
		Key: key,
	}

	resp, err := c.sendCommand(cmd)
	if err != nil {
		return "", 0, err
	}

	if resp.Status != "success" {
		return "", 0, fmt.Errorf("server error: %s", resp.Message)
	}

	return resp.Value, resp.TTL, nil
}

func (c *RaftClient) Delete(key string) error {
	cmd := Command{
		Op:  "DELETE",
		Key: key,
	}

	for retry := 0; retry <= c.maxRetries; retry++ {
		resp, err := c.sendCommand(cmd)
		if err != nil {
			return err
		}

		if resp.Status == "success" {
			return nil
		} else if resp.Status == "redirect" {
			newAddr := extractServerAddress(resp.Message)
			if newAddr != "" && newAddr != c.serverAddr {
				if err := c.reconnectToServer(newAddr); err != nil {
					return err
				}
				continue
			}
		}

		return fmt.Errorf("server error: %s", resp.Message)
	}

	return fmt.Errorf("max retries reached")
}

func (c *RaftClient) TTL(key string) (time.Duration, error) {
	cmd := Command{
		Op:  "TTL",
		Key: key,
	}

	resp, err := c.sendCommand(cmd)
	if err != nil {
		return 0, err
	}

	if resp.Status != "success" {
		return 0, fmt.Errorf("server error: %s", resp.Message)
	}

	return resp.TTL, nil
}

func (c *RaftClient) Status() (string, error) {
	cmd := Command{
		Op: "STATUS",
	}

	resp, err := c.sendCommand(cmd)
	if err != nil {
		return "", err
	}

	if resp.Status != "success" {
		return "", fmt.Errorf("server error: %s", resp.Message)
	}

	return resp.Message, nil
}

func (c *RaftClient) reconnectToServer(serverAddr string) error {
	// Close current connection
	c.conn.Close()

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to server at %s: %w", serverAddr, err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.serverAddr = serverAddr

	return nil
}

func extractServerAddress(message string) string {
	if strings.Contains(message, "try:") {
		parts := strings.Split(message, "try:")
		if len(parts) >= 2 {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func (c *RaftClient) sendCommand(cmd Command) (*Response, error) {
	jsonCmd, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	jsonCmd = append(jsonCmd, '\n')

	_, err = c.conn.Write(jsonCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Read response
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var resp Response
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}
