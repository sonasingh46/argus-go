// Package store defines interfaces for data persistence and state management.
// These abstractions allow swapping implementations (Redis, PostgreSQL, in-memory)
// without changing business logic.
package store

import (
	"context"
	"time"
)

// ParentState represents the cached state of a parent alert in the state store.
// This is used for fast lookups during event processing.
type ParentState struct {
	// DedupKey is the deduplication key of the parent alert.
	DedupKey string `json:"dedupKey"`

	// CreatedAt is when the parent alert was created.
	CreatedAt time.Time `json:"created_at"`

	// ChildCount is the current number of children linked to this parent.
	ChildCount int `json:"child_count"`
}

// AlertState represents the cached state of any alert (parent or child).
type AlertState struct {
	// DedupKey is the deduplication key of the alert.
	DedupKey string `json:"dedupKey"`

	// EventManagerID identifies the namespace this alert belongs to.
	EventManagerID string `json:"event_manager_id"`

	// Type is "parent" or "child".
	Type string `json:"type"`

	// Status is "active" or "resolved".
	Status string `json:"status"`

	// ParentDedupKey is set for child alerts to reference their parent.
	ParentDedupKey string `json:"parent_dedupKey,omitempty"`

	// ResolveRequested indicates if a resolve was requested for this alert.
	ResolveRequested bool `json:"resolve_requested"`
}

// PendingResolve tracks a parent alert waiting for children to resolve.
type PendingResolve struct {
	// RequestedAt is when the resolve was first requested.
	RequestedAt time.Time `json:"requested_at"`

	// RemainingChildren is how many children still need to resolve.
	RemainingChildren int `json:"remaining_children"`
}

// StateStore defines the interface for fast in-memory state operations.
// This is typically backed by Redis for production use.
// All methods must be safe for concurrent use.
type StateStore interface {
	// --- Parent Alert Operations ---

	// GetParent retrieves the parent state for a given grouping combination.
	// Returns nil, nil if no parent exists.
	GetParent(ctx context.Context, eventManagerID, groupingKey, groupingValue string) (*ParentState, error)

	// SetParent stores a parent state with the specified TTL.
	// The TTL should match the grouping rule's time window.
	SetParent(ctx context.Context, eventManagerID, groupingKey, groupingValue string, state *ParentState, ttl time.Duration) error

	// DeleteParent removes a parent state entry.
	DeleteParent(ctx context.Context, eventManagerID, groupingKey, groupingValue string) error

	// --- Alert State Operations ---

	// GetAlert retrieves the state for an alert by its dedup key.
	// Returns nil, nil if the alert doesn't exist.
	GetAlert(ctx context.Context, dedupKey string) (*AlertState, error)

	// SetAlert stores or updates an alert's state.
	SetAlert(ctx context.Context, state *AlertState) error

	// DeleteAlert removes an alert state entry.
	DeleteAlert(ctx context.Context, dedupKey string) error

	// --- Parent-Child Relationship Operations ---

	// AddChild adds a child dedup key to a parent's children set.
	AddChild(ctx context.Context, parentDedupKey, childDedupKey string) error

	// RemoveChild removes a child from a parent's children set.
	RemoveChild(ctx context.Context, parentDedupKey, childDedupKey string) error

	// GetChildren returns all child dedup keys for a parent.
	GetChildren(ctx context.Context, parentDedupKey string) ([]string, error)

	// GetChildCount returns the number of children for a parent.
	GetChildCount(ctx context.Context, parentDedupKey string) (int, error)

	// --- Pending Resolution Operations ---

	// SetPendingResolve marks a parent as having a pending resolve request.
	SetPendingResolve(ctx context.Context, parentDedupKey string, pending *PendingResolve) error

	// GetPendingResolve retrieves pending resolve info for a parent.
	// Returns nil, nil if no pending resolve exists.
	GetPendingResolve(ctx context.Context, parentDedupKey string) (*PendingResolve, error)

	// DeletePendingResolve removes a pending resolve entry.
	DeletePendingResolve(ctx context.Context, parentDedupKey string) error

	// --- Lifecycle ---

	// Close releases any resources held by the store.
	Close() error
}
