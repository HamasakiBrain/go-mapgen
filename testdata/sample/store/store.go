// Package store provides an in-memory key/value store.
package store

import (
	"errors"
	"sync"
)

// ErrNotFound is returned when a key is missing.
var ErrNotFound = errors.New("not found")

// MemStore is a concurrency-safe in-memory store. It structurally satisfies
// the api.Store interface without importing the api package.
type MemStore struct {
	mu   sync.RWMutex
	data map[string]string
}

// NewMemStore constructs an empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{data: make(map[string]string)}
}

// Get returns the value for id.
func (m *MemStore) Get(id string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[id]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

// Save stores val under id.
func (m *MemStore) Save(id, val string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[id] = val
	return nil
}
