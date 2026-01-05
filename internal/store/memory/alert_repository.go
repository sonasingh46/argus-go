package memory

import (
	"context"
	"sync"

	"argus-go/internal/domain"
)

// AlertRepository is an in-memory implementation of store.AlertRepository.
// It stores alerts in a map, indexed by both ID and DedupKey for fast lookups.
type AlertRepository struct {
	mu sync.RWMutex

	// alerts stores all alerts by their database ID
	alerts map[string]*domain.Alert

	// byDedupKey provides fast lookup by dedup key
	byDedupKey map[string]*domain.Alert

	// byParent provides fast lookup of children by parent dedup key
	byParent map[string]map[string]*domain.Alert
}

// NewAlertRepository creates a new in-memory alert repository.
func NewAlertRepository() *AlertRepository {
	return &AlertRepository{
		alerts:     make(map[string]*domain.Alert),
		byDedupKey: make(map[string]*domain.Alert),
		byParent:   make(map[string]map[string]*domain.Alert),
	}
}

// Create stores a new alert.
func (r *AlertRepository) Create(ctx context.Context, alert *domain.Alert) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Store a copy to prevent external modification
	alertCopy := *alert
	r.alerts[alert.ID] = &alertCopy
	r.byDedupKey[alert.DedupKey] = &alertCopy

	// Index children by parent
	if alert.IsChild() && alert.ParentDedupKey != "" {
		if r.byParent[alert.ParentDedupKey] == nil {
			r.byParent[alert.ParentDedupKey] = make(map[string]*domain.Alert)
		}
		r.byParent[alert.ParentDedupKey][alert.DedupKey] = &alertCopy
	}

	return nil
}

// Update modifies an existing alert.
func (r *AlertRepository) Update(ctx context.Context, alert *domain.Alert) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.alerts[alert.ID]
	if !exists {
		return domain.ErrAlertNotFound
	}

	// Store a copy
	alertCopy := *alert
	r.alerts[alert.ID] = &alertCopy
	r.byDedupKey[alert.DedupKey] = &alertCopy

	// Update parent index if needed
	if alert.IsChild() && alert.ParentDedupKey != "" {
		if r.byParent[alert.ParentDedupKey] == nil {
			r.byParent[alert.ParentDedupKey] = make(map[string]*domain.Alert)
		}
		r.byParent[alert.ParentDedupKey][alert.DedupKey] = &alertCopy
	}

	// Handle parent change (unlikely but handle it)
	if existing.ParentDedupKey != "" && existing.ParentDedupKey != alert.ParentDedupKey {
		delete(r.byParent[existing.ParentDedupKey], existing.DedupKey)
	}

	return nil
}

// GetByID retrieves an alert by its database ID.
func (r *AlertRepository) GetByID(ctx context.Context, id string) (*domain.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	alert, exists := r.alerts[id]
	if !exists {
		return nil, domain.ErrAlertNotFound
	}

	// Return a copy
	result := *alert
	return &result, nil
}

// GetByDedupKey retrieves an alert by its deduplication key.
func (r *AlertRepository) GetByDedupKey(ctx context.Context, dedupKey string) (*domain.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	alert, exists := r.byDedupKey[dedupKey]
	if !exists {
		return nil, domain.ErrAlertNotFound
	}

	// Return a copy
	result := *alert
	return &result, nil
}

// List retrieves alerts matching the filter criteria.
func (r *AlertRepository) List(ctx context.Context, filter domain.AlertFilter) ([]*domain.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*domain.Alert

	for _, alert := range r.alerts {
		// Apply filters
		if filter.EventManagerID != "" && alert.EventManagerID != filter.EventManagerID {
			continue
		}
		if filter.Status != "" && alert.Status != filter.Status {
			continue
		}
		if filter.Type != "" && alert.Type != filter.Type {
			continue
		}

		// Return a copy
		alertCopy := *alert
		results = append(results, &alertCopy)
	}

	// Apply offset and limit
	start := filter.Offset
	if start > len(results) {
		start = len(results)
	}

	end := len(results)
	if filter.Limit > 0 && start+filter.Limit < end {
		end = start + filter.Limit
	}

	return results[start:end], nil
}

// GetChildrenByParent retrieves all child alerts for a given parent dedup key.
func (r *AlertRepository) GetChildrenByParent(ctx context.Context, parentDedupKey string) ([]*domain.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	children := r.byParent[parentDedupKey]
	if children == nil {
		return []*domain.Alert{}, nil
	}

	results := make([]*domain.Alert, 0, len(children))
	for _, alert := range children {
		alertCopy := *alert
		results = append(results, &alertCopy)
	}

	return results, nil
}

// CountActiveChildren returns the count of active child alerts for a parent.
func (r *AlertRepository) CountActiveChildren(ctx context.Context, parentDedupKey string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	children := r.byParent[parentDedupKey]
	if children == nil {
		return 0, nil
	}

	count := 0
	for _, alert := range children {
		if alert.IsActive() {
			count++
		}
	}

	return count, nil
}

// Clear removes all data from the repository. Useful for test cleanup.
func (r *AlertRepository) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.alerts = make(map[string]*domain.Alert)
	r.byDedupKey = make(map[string]*domain.Alert)
	r.byParent = make(map[string]map[string]*domain.Alert)
}
