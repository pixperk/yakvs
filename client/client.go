package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type Client struct {
	conn   net.Conn
	reader *bufio.Reader
}

type Command struct {
	Op        string        `json:"op"`
	Key       string        `json:"key"`
	Value     string        `json:"value,omitempty"`
	ExpiresIn time.Duration `json:"expires_in,omitempty"`
}

type Response struct {
	Status  string        `json:"status"`
	Message string        `json:"message,omitempty"`
	Value   string        `json:"value,omitempty"`
	TTL     time.Duration `json:"ttl,omitempty"`
}

func NewClient(serverAddr string) (*Client, error) {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server at %s: %w", serverAddr, err)
	}

	return &Client{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Set(key, value string, expiresIn time.Duration) error {
	cmd := Command{
		Op:        "SET",
		Key:       key,
		Value:     value,
		ExpiresIn: expiresIn,
	}

	resp, err := c.sendCommand(cmd)
	if err != nil {
		return err
	}

	if resp.Status != "success" {
		return fmt.Errorf("server error: %s", resp.Message)
	}

	return nil
}

func (c *Client) Get(key string) (string, time.Duration, error) {
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

func (c *Client) Delete(key string) error {
	cmd := Command{
		Op:  "DELETE",
		Key: key,
	}

	resp, err := c.sendCommand(cmd)
	if err != nil {
		return err
	}

	if resp.Status != "success" {
		return fmt.Errorf("server error: %s", resp.Message)
	}

	return nil
}

func (c *Client) TTL(key string) (time.Duration, error) {
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

func (c *Client) sendCommand(cmd Command) (*Response, error) {
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
