// Package redis provides Redis-based implementations of the store interfaces.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"argus-go/internal/config"
	"argus-go/internal/store"
)

// Key prefixes for different data types in Redis.
const (
	prefixParent         = "parent:"
	prefixAlert          = "alert:"
	prefixChildren       = "children:"
	prefixPendingResolve = "pending:"
)

// StateStore implements store.StateStore using Redis.
type StateStore struct {
	client *redis.Client
}

// NewStateStore creates a new Redis-backed state store.
func NewStateStore(cfg *config.RedisConfig) (*StateStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &StateStore{client: client}, nil
}

// --- Parent Alert Operations ---

// parentKey generates the Redis key for a parent state.
func parentKey(eventManagerID, groupingKey, groupingValue string) string {
	return fmt.Sprintf("%s%s:%s:%s", prefixParent, eventManagerID, groupingKey, groupingValue)
}

// GetParent retrieves the parent state for a given grouping combination.
func (s *StateStore) GetParent(ctx context.Context, eventManagerID, groupingKey, groupingValue string) (*store.ParentState, error) {
	key := parentKey(eventManagerID, groupingKey, groupingValue)

	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get parent: %w", err)
	}

	var state store.ParentState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal parent state: %w", err)
	}

	return &state, nil
}

// SetParent stores a parent state with the specified TTL.
func (s *StateStore) SetParent(ctx context.Context, eventManagerID, groupingKey, groupingValue string, state *store.ParentState, ttl time.Duration) error {
	key := parentKey(eventManagerID, groupingKey, groupingValue)

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal parent state: %w", err)
	}

	if err := s.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set parent: %w", err)
	}

	return nil
}

// DeleteParent removes a parent state entry.
func (s *StateStore) DeleteParent(ctx context.Context, eventManagerID, groupingKey, groupingValue string) error {
	key := parentKey(eventManagerID, groupingKey, groupingValue)

	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete parent: %w", err)
	}

	return nil
}

// --- Alert State Operations ---

// alertKey generates the Redis key for an alert state.
func alertKey(dedupKey string) string {
	return prefixAlert + dedupKey
}

// GetAlert retrieves the state for an alert by its dedup key.
func (s *StateStore) GetAlert(ctx context.Context, dedupKey string) (*store.AlertState, error) {
	key := alertKey(dedupKey)

	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get alert: %w", err)
	}

	var state store.AlertState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal alert state: %w", err)
	}

	return &state, nil
}

// SetAlert stores or updates an alert's state.
func (s *StateStore) SetAlert(ctx context.Context, state *store.AlertState) error {
	key := alertKey(state.DedupKey)

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal alert state: %w", err)
	}

	// No TTL for alert state - it persists until explicitly deleted
	if err := s.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to set alert: %w", err)
	}

	return nil
}

// DeleteAlert removes an alert state entry.
func (s *StateStore) DeleteAlert(ctx context.Context, dedupKey string) error {
	key := alertKey(dedupKey)

	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete alert: %w", err)
	}

	return nil
}

// --- Parent-Child Relationship Operations ---

// childrenKey generates the Redis key for a parent's children set.
func childrenKey(parentDedupKey string) string {
	return prefixChildren + parentDedupKey
}

// AddChild adds a child dedup key to a parent's children set.
func (s *StateStore) AddChild(ctx context.Context, parentDedupKey, childDedupKey string) error {
	key := childrenKey(parentDedupKey)

	if err := s.client.SAdd(ctx, key, childDedupKey).Err(); err != nil {
		return fmt.Errorf("failed to add child: %w", err)
	}

	return nil
}

// RemoveChild removes a child from a parent's children set.
func (s *StateStore) RemoveChild(ctx context.Context, parentDedupKey, childDedupKey string) error {
	key := childrenKey(parentDedupKey)

	if err := s.client.SRem(ctx, key, childDedupKey).Err(); err != nil {
		return fmt.Errorf("failed to remove child: %w", err)
	}

	return nil
}

// GetChildren returns all child dedup keys for a parent.
func (s *StateStore) GetChildren(ctx context.Context, parentDedupKey string) ([]string, error) {
	key := childrenKey(parentDedupKey)

	children, err := s.client.SMembers(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get children: %w", err)
	}

	return children, nil
}

// GetChildCount returns the number of children for a parent.
func (s *StateStore) GetChildCount(ctx context.Context, parentDedupKey string) (int, error) {
	key := childrenKey(parentDedupKey)

	count, err := s.client.SCard(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get child count: %w", err)
	}

	return int(count), nil
}

// --- Pending Resolution Operations ---

// pendingKey generates the Redis key for pending resolve state.
func pendingKey(parentDedupKey string) string {
	return prefixPendingResolve + parentDedupKey
}

// SetPendingResolve marks a parent as having a pending resolve request.
func (s *StateStore) SetPendingResolve(ctx context.Context, parentDedupKey string, pending *store.PendingResolve) error {
	key := pendingKey(parentDedupKey)

	data, err := json.Marshal(pending)
	if err != nil {
		return fmt.Errorf("failed to marshal pending resolve: %w", err)
	}

	if err := s.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to set pending resolve: %w", err)
	}

	return nil
}

// GetPendingResolve retrieves pending resolve info for a parent.
func (s *StateStore) GetPendingResolve(ctx context.Context, parentDedupKey string) (*store.PendingResolve, error) {
	key := pendingKey(parentDedupKey)

	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get pending resolve: %w", err)
	}

	var pending store.PendingResolve
	if err := json.Unmarshal(data, &pending); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pending resolve: %w", err)
	}

	return &pending, nil
}

// DeletePendingResolve removes a pending resolve entry.
func (s *StateStore) DeletePendingResolve(ctx context.Context, parentDedupKey string) error {
	key := pendingKey(parentDedupKey)

	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete pending resolve: %w", err)
	}

	return nil
}

// --- Lifecycle ---

// Close closes the Redis client connection.
func (s *StateStore) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}
