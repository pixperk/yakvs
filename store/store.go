package store

import (
	"bufio"
	"os"
	"strings"
	"sync"
	"time"
)

// Store provides a persistent key-value store with expiration
type Store struct {
	mu   sync.RWMutex
	data map[string]Value
	log  *os.File
}

type Value struct {
	Data      string
	ExpiresAt time.Time
}

func NewStore(logFilePath string) (*Store, error) {

	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}

	s := &Store{
		data: make(map[string]Value),
		log:  logFile,
	}

	s.ReplayLogs()

	return s, nil
}

func NewValue(data string, expiresAfter time.Duration) Value {
	expiresAt := time.Now().Add(expiresAfter)
	val := Value{
		Data:      data,
		ExpiresAt: expiresAt,
	}

	return val
}

func (s *Store) Set(key string, value Value) {
	s.mu.Lock()
	defer s.mu.Unlock()

	//append to log with expiry timestamp
	expiryTimestamp := value.ExpiresAt.Format(time.RFC3339)
	_, err := s.log.WriteString(time.Now().Format(time.RFC3339) + " SET " + key + " " + expiryTimestamp + " " + value.Data + "\n")
	if err != nil {
		return
	}
	s.data[key] = value
}

func (s *Store) Get(key string) (Value, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.data[key]
	if val.ExpiresAt.Before(time.Now()) {
		return Value{}, false
	}
	return val, ok
}

func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	//append to log
	_, err := s.log.WriteString(time.Now().Format(time.RFC3339) + " DELETE " + key + "\n")
	if err != nil {
		return
	}
	delete(s.data, key)
}

// ReplayLogs rebuilds the store's in-memory data by replaying all operations from the log file.
// This should only be called during initialization, before any concurrent access to the store.
func (s *Store) ReplayLogs() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log.Seek(0, 0)

	s.data = make(map[string]Value)

	scanner := bufio.NewScanner(s.log)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, " ")

		if len(parts) < 3 {
			continue
		}

		operation := parts[1]
		key := parts[2]

		switch operation {
		case "SET":
			if len(parts) < 5 {
				continue // Need at least timestamp, operation, key, expiry, and data
			}

			expiryTimestamp := parts[3]
			data := strings.Join(parts[4:], " ")

			// Parse the expiry timestamp
			expiresAt, err := time.Parse(time.RFC3339, expiryTimestamp)
			if err != nil {
				continue
			}

			s.data[key] = Value{
				Data:      data,
				ExpiresAt: expiresAt,
			}

		case "DELETE":
			delete(s.data, key)
		}
	}
	if err := scanner.Err(); err != nil {
		// In a real implementation, you might want to log this error
		return
	}
}

func (s *Store) TTL(key string) (time.Duration, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.data[key]
	if !ok || val.ExpiresAt.Before(time.Now()) {
		return 0, false
	}

	ttl := time.Until(val.ExpiresAt)
	return ttl, true
}

func (s *Store) BackgroundCleaner() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, val := range s.data {
		if val.ExpiresAt.Before(now) {
			delete(s.data, key)

			_, err := s.log.WriteString(time.Now().Format(time.RFC3339) + " DELETE " + key + "\n")
			if err != nil {
				// In a real implementation, you might want to log this error
				continue
			}
		}
	}
}

func (s *Store) StartBackgroundCleaner() {
	go func() {
		for {
			time.Sleep(10 * time.Second)
			s.BackgroundCleaner()
		}
	}()
}

// Range iterates over all key-value pairs in the store, calling fn for each
func (s *Store) Range(fn func(key string, value Value) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for k, v := range s.data {
		if !fn(k, v) {
			break
		}
	}
}

// Clear removes all key-value pairs from the store
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = make(map[string]Value)
}
