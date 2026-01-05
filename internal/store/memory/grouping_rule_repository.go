package memory

import (
	"context"
	"sync"

	"argus-go/internal/domain"
)

// GroupingRuleRepository is an in-memory implementation of store.GroupingRuleRepository.
type GroupingRuleRepository struct {
	mu sync.RWMutex

	// groupingRules stores all grouping rules by their ID
	groupingRules map[string]*domain.GroupingRule
}

// NewGroupingRuleRepository creates a new in-memory grouping rule repository.
func NewGroupingRuleRepository() *GroupingRuleRepository {
	return &GroupingRuleRepository{
		groupingRules: make(map[string]*domain.GroupingRule),
	}
}

// Create stores a new grouping rule.
func (r *GroupingRuleRepository) Create(ctx context.Context, rule *domain.GroupingRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Store a copy
	ruleCopy := *rule
	r.groupingRules[rule.ID] = &ruleCopy
	return nil
}

// Update modifies an existing grouping rule.
func (r *GroupingRuleRepository) Update(ctx context.Context, rule *domain.GroupingRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.groupingRules[rule.ID]; !exists {
		return domain.ErrGroupingRuleNotFound
	}

	// Store a copy
	ruleCopy := *rule
	r.groupingRules[rule.ID] = &ruleCopy
	return nil
}

// Delete removes a grouping rule by ID.
func (r *GroupingRuleRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.groupingRules[id]; !exists {
		return domain.ErrGroupingRuleNotFound
	}

	delete(r.groupingRules, id)
	return nil
}

// GetByID retrieves a grouping rule by its ID.
func (r *GroupingRuleRepository) GetByID(ctx context.Context, id string) (*domain.GroupingRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rule, exists := r.groupingRules[id]
	if !exists {
		return nil, domain.ErrGroupingRuleNotFound
	}

	// Return a copy
	result := *rule
	return &result, nil
}

// List retrieves all grouping rules.
func (r *GroupingRuleRepository) List(ctx context.Context) ([]*domain.GroupingRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]*domain.GroupingRule, 0, len(r.groupingRules))
	for _, rule := range r.groupingRules {
		ruleCopy := *rule
		results = append(results, &ruleCopy)
	}

	return results, nil
}

// Clear removes all data from the repository. Useful for test cleanup.
func (r *GroupingRuleRepository) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.groupingRules = make(map[string]*domain.GroupingRule)
}
