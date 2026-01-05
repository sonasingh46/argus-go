package memory

import (
	"context"
	"sync"

	"argus-go/internal/domain"
)

// EventManagerRepository is an in-memory implementation of store.EventManagerRepository.
type EventManagerRepository struct {
	mu sync.RWMutex

	// eventManagers stores all event managers by their ID
	eventManagers map[string]*domain.EventManager
}

// NewEventManagerRepository creates a new in-memory event manager repository.
func NewEventManagerRepository() *EventManagerRepository {
	return &EventManagerRepository{
		eventManagers: make(map[string]*domain.EventManager),
	}
}

// Create stores a new event manager.
func (r *EventManagerRepository) Create(ctx context.Context, em *domain.EventManager) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.eventManagers[em.ID]; exists {
		return domain.ErrEventManagerAlreadyExists
	}

	// Store a copy
	emCopy := *em
	r.eventManagers[em.ID] = &emCopy
	return nil
}

// Update modifies an existing event manager.
func (r *EventManagerRepository) Update(ctx context.Context, em *domain.EventManager) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.eventManagers[em.ID]; !exists {
		return domain.ErrEventManagerNotFound
	}

	// Store a copy
	emCopy := *em
	r.eventManagers[em.ID] = &emCopy
	return nil
}

// Delete removes an event manager by ID.
func (r *EventManagerRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.eventManagers[id]; !exists {
		return domain.ErrEventManagerNotFound
	}

	delete(r.eventManagers, id)
	return nil
}

// GetByID retrieves an event manager by its ID.
func (r *EventManagerRepository) GetByID(ctx context.Context, id string) (*domain.EventManager, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	em, exists := r.eventManagers[id]
	if !exists {
		return nil, domain.ErrEventManagerNotFound
	}

	// Return a copy
	result := *em
	return &result, nil
}

// List retrieves all event managers.
func (r *EventManagerRepository) List(ctx context.Context) ([]*domain.EventManager, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]*domain.EventManager, 0, len(r.eventManagers))
	for _, em := range r.eventManagers {
		emCopy := *em
		results = append(results, &emCopy)
	}

	return results, nil
}

// Clear removes all data from the repository. Useful for test cleanup.
func (r *EventManagerRepository) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.eventManagers = make(map[string]*domain.EventManager)
}
