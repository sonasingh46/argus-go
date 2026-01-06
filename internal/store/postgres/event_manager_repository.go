package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"argus-go/internal/domain"
)

// EventManagerRepository implements store.EventManagerRepository using PostgreSQL.
type EventManagerRepository struct {
	db *DB
}

// NewEventManagerRepository creates a new PostgreSQL-backed event manager repository.
func NewEventManagerRepository(db *DB) *EventManagerRepository {
	return &EventManagerRepository{db: db}
}

// Create stores a new event manager.
func (r *EventManagerRepository) Create(ctx context.Context, em *domain.EventManager) error {
	query := `
		INSERT INTO event_managers (
			id, name, description, grouping_rule_id, webhook_url, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.pool.Exec(ctx, query,
		em.ID,
		em.Name,
		em.Description,
		em.GroupingRuleID,
		em.NotificationConfig.WebhookURL,
		em.CreatedAt,
		em.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create event manager: %w", err)
	}

	return nil
}

// Update modifies an existing event manager.
func (r *EventManagerRepository) Update(ctx context.Context, em *domain.EventManager) error {
	query := `
		UPDATE event_managers SET
			name = $2,
			description = $3,
			grouping_rule_id = $4,
			webhook_url = $5,
			updated_at = $6
		WHERE id = $1
	`

	result, err := r.db.pool.Exec(ctx, query,
		em.ID,
		em.Name,
		em.Description,
		em.GroupingRuleID,
		em.NotificationConfig.WebhookURL,
		em.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update event manager: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrEventManagerNotFound
	}

	return nil
}

// Delete removes an event manager by ID.
func (r *EventManagerRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM event_managers WHERE id = $1`

	result, err := r.db.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete event manager: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrEventManagerNotFound
	}

	return nil
}

// GetByID retrieves an event manager by its ID.
func (r *EventManagerRepository) GetByID(ctx context.Context, id string) (*domain.EventManager, error) {
	query := `
		SELECT id, name, description, grouping_rule_id, webhook_url, created_at, updated_at
		FROM event_managers
		WHERE id = $1
	`

	row := r.db.pool.QueryRow(ctx, query, id)

	em, err := scanEventManager(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrEventManagerNotFound
		}
		return nil, fmt.Errorf("failed to get event manager: %w", err)
	}

	return em, nil
}

// List retrieves all event managers.
func (r *EventManagerRepository) List(ctx context.Context) ([]*domain.EventManager, error) {
	query := `
		SELECT id, name, description, grouping_rule_id, webhook_url, created_at, updated_at
		FROM event_managers
		ORDER BY created_at DESC
	`

	rows, err := r.db.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list event managers: %w", err)
	}
	defer rows.Close()

	var managers []*domain.EventManager

	for rows.Next() {
		em, err := scanEventManagerRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event manager: %w", err)
		}
		managers = append(managers, em)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating event managers: %w", err)
	}

	return managers, nil
}

// scanEventManager scans a single row into an EventManager.
func scanEventManager(row pgx.Row) (*domain.EventManager, error) {
	var em domain.EventManager
	var webhookURL *string

	err := row.Scan(
		&em.ID,
		&em.Name,
		&em.Description,
		&em.GroupingRuleID,
		&webhookURL,
		&em.CreatedAt,
		&em.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if webhookURL != nil {
		em.NotificationConfig.WebhookURL = *webhookURL
	}

	return &em, nil
}

// scanEventManagerRow scans a row from a Rows iterator into an EventManager.
func scanEventManagerRow(rows pgx.Rows) (*domain.EventManager, error) {
	var em domain.EventManager
	var webhookURL *string

	err := rows.Scan(
		&em.ID,
		&em.Name,
		&em.Description,
		&em.GroupingRuleID,
		&webhookURL,
		&em.CreatedAt,
		&em.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if webhookURL != nil {
		em.NotificationConfig.WebhookURL = *webhookURL
	}

	return &em, nil
}
