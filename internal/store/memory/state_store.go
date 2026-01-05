// Package memory provides in-memory implementations of store interfaces.
// These are useful for testing and development without external dependencies.
package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"argus-go/internal/store"
)

// StateStore is an in-memory implementation of the store.StateStore interface.
// It uses maps with mutex protection for thread-safe access.
// TTL expiration is checked on access (lazy expiration).
type StateStore struct {
	mu sync.RWMutex

	// parents stores parent state keyed by "eventManagerID:groupingKey:groupingValue"
	parents map[string]*parentEntry

	// alerts stores alert state keyed by dedupKey
	alerts map[string]*store.AlertState

	// children stores parent -> set of child dedupKeys
	children map[string]map[string]struct{}

	// pendingResolves stores pending resolution info by parent dedupKey
	pendingResolves map[string]*store.PendingResolve
}

// parentEntry wraps ParentState with expiration tracking.
type parentEntry struct {
	state     *store.ParentState
	expiresAt time.Time
}

// NewStateStore creates a new in-memory state store.
func NewStateStore() *StateStore {
	return &StateStore{
		parents:         make(map[string]*parentEntry),
		alerts:          make(map[string]*store.AlertState),
		children:        make(map[string]map[string]struct{}),
		pendingResolves: make(map[string]*store.PendingResolve),
	}
}

// parentKey generates the key for parent lookup.
func parentKey(eventManagerID, groupingKey, groupingValue string) string {
	return fmt.Sprintf("%s:%s:%s", eventManagerID, groupingKey, groupingValue)
}

// --- Parent Alert Operations ---

// GetParent retrieves the parent state for a given grouping combination.
// Returns nil, nil if no parent exists or if the entry has expired.
func (s *StateStore) GetParent(ctx context.Context, eventManagerID, groupingKey, groupingValue string) (*store.ParentState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := parentKey(eventManagerID, groupingKey, groupingValue)
	entry, exists := s.parents[key]
	if !exists {
		return nil, nil
	}

	// Check if expired (lazy expiration)
	if time.Now().After(entry.expiresAt) {
		return nil, nil
	}

	// Return a copy to prevent external modification
	result := *entry.state
	return &result, nil
}

// SetParent stores a parent state with the specified TTL.
func (s *StateStore) SetParent(ctx context.Context, eventManagerID, groupingKey, groupingValue string, state *store.ParentState, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := parentKey(eventManagerID, groupingKey, groupingValue)
	// Store a copy to prevent external modification
	stateCopy := *state
	s.parents[key] = &parentEntry{
		state:     &stateCopy,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

// DeleteParent removes a parent state entry.
func (s *StateStore) DeleteParent(ctx context.Context, eventManagerID, groupingKey, groupingValue string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := parentKey(eventManagerID, groupingKey, groupingValue)
	delete(s.parents, key)
	return nil
}

// --- Alert State Operations ---

// GetAlert retrieves the state for an alert by its dedup key.
func (s *StateStore) GetAlert(ctx context.Context, dedupKey string) (*store.AlertState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, exists := s.alerts[dedupKey]
	if !exists {
		return nil, nil
	}

	// Return a copy
	result := *state
	return &result, nil
}

// SetAlert stores or updates an alert's state.
func (s *StateStore) SetAlert(ctx context.Context, state *store.AlertState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a copy
	stateCopy := *state
	s.alerts[state.DedupKey] = &stateCopy
	return nil
}

// DeleteAlert removes an alert state entry.
func (s *StateStore) DeleteAlert(ctx context.Context, dedupKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.alerts, dedupKey)
	return nil
}

// --- Parent-Child Relationship Operations ---

// AddChild adds a child dedup key to a parent's children set.
func (s *StateStore) AddChild(ctx context.Context, parentDedupKey, childDedupKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.children[parentDedupKey] == nil {
		s.children[parentDedupKey] = make(map[string]struct{})
	}
	s.children[parentDedupKey][childDedupKey] = struct{}{}
	return nil
}

// RemoveChild removes a child from a parent's children set.
func (s *StateStore) RemoveChild(ctx context.Context, parentDedupKey, childDedupKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.children[parentDedupKey] != nil {
		delete(s.children[parentDedupKey], childDedupKey)
	}
	return nil
}

// GetChildren returns all child dedup keys for a parent.
func (s *StateStore) GetChildren(ctx context.Context, parentDedupKey string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	childSet := s.children[parentDedupKey]
	if childSet == nil {
		return []string{}, nil
	}

	result := make([]string, 0, len(childSet))
	for childKey := range childSet {
		result = append(result, childKey)
	}
	return result, nil
}

// GetChildCount returns the number of children for a parent.
func (s *StateStore) GetChildCount(ctx context.Context, parentDedupKey string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.children[parentDedupKey]), nil
}

// --- Pending Resolution Operations ---

// SetPendingResolve marks a parent as having a pending resolve request.
func (s *StateStore) SetPendingResolve(ctx context.Context, parentDedupKey string, pending *store.PendingResolve) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a copy
	pendingCopy := *pending
	s.pendingResolves[parentDedupKey] = &pendingCopy
	return nil
}

// GetPendingResolve retrieves pending resolve info for a parent.
func (s *StateStore) GetPendingResolve(ctx context.Context, parentDedupKey string) (*store.PendingResolve, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pending, exists := s.pendingResolves[parentDedupKey]
	if !exists {
		return nil, nil
	}

	// Return a copy
	result := *pending
	return &result, nil
}

// DeletePendingResolve removes a pending resolve entry.
func (s *StateStore) DeletePendingResolve(ctx context.Context, parentDedupKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.pendingResolves, parentDedupKey)
	return nil
}

// Close releases any resources (no-op for in-memory store).
func (s *StateStore) Close() error {
	return nil
}

// --- Test Helpers ---

// Clear removes all data from the store. Useful for test cleanup.
func (s *StateStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.parents = make(map[string]*parentEntry)
	s.alerts = make(map[string]*store.AlertState)
	s.children = make(map[string]map[string]struct{})
	s.pendingResolves = make(map[string]*store.PendingResolve)
}
