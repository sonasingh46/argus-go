package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"argus-go/internal/domain"
)

// GroupingRuleRepository implements store.GroupingRuleRepository using PostgreSQL.
type GroupingRuleRepository struct {
	db *DB
}

// NewGroupingRuleRepository creates a new PostgreSQL-backed grouping rule repository.
func NewGroupingRuleRepository(db *DB) *GroupingRuleRepository {
	return &GroupingRuleRepository{db: db}
}

// Create stores a new grouping rule.
func (r *GroupingRuleRepository) Create(ctx context.Context, rule *domain.GroupingRule) error {
	query := `
		INSERT INTO grouping_rules (
			id, name, grouping_key, time_window_minutes, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.pool.Exec(ctx, query,
		rule.ID,
		rule.Name,
		rule.GroupingKey,
		rule.TimeWindowMinutes,
		rule.CreatedAt,
		rule.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create grouping rule: %w", err)
	}

	return nil
}

// Update modifies an existing grouping rule.
func (r *GroupingRuleRepository) Update(ctx context.Context, rule *domain.GroupingRule) error {
	query := `
		UPDATE grouping_rules SET
			name = $2,
			grouping_key = $3,
			time_window_minutes = $4,
			updated_at = $5
		WHERE id = $1
	`

	result, err := r.db.pool.Exec(ctx, query,
		rule.ID,
		rule.Name,
		rule.GroupingKey,
		rule.TimeWindowMinutes,
		rule.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update grouping rule: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrGroupingRuleNotFound
	}

	return nil
}

// Delete removes a grouping rule by ID.
func (r *GroupingRuleRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM grouping_rules WHERE id = $1`

	result, err := r.db.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete grouping rule: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrGroupingRuleNotFound
	}

	return nil
}

// GetByID retrieves a grouping rule by its ID.
func (r *GroupingRuleRepository) GetByID(ctx context.Context, id string) (*domain.GroupingRule, error) {
	query := `
		SELECT id, name, grouping_key, time_window_minutes, created_at, updated_at
		FROM grouping_rules
		WHERE id = $1
	`

	row := r.db.pool.QueryRow(ctx, query, id)

	rule, err := scanGroupingRule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrGroupingRuleNotFound
		}
		return nil, fmt.Errorf("failed to get grouping rule: %w", err)
	}

	return rule, nil
}

// List retrieves all grouping rules.
func (r *GroupingRuleRepository) List(ctx context.Context) ([]*domain.GroupingRule, error) {
	query := `
		SELECT id, name, grouping_key, time_window_minutes, created_at, updated_at
		FROM grouping_rules
		ORDER BY created_at DESC
	`

	rows, err := r.db.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list grouping rules: %w", err)
	}
	defer rows.Close()

	var rules []*domain.GroupingRule

	for rows.Next() {
		rule, err := scanGroupingRuleRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan grouping rule: %w", err)
		}
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating grouping rules: %w", err)
	}

	return rules, nil
}

// scanGroupingRule scans a single row into a GroupingRule.
func scanGroupingRule(row pgx.Row) (*domain.GroupingRule, error) {
	var rule domain.GroupingRule

	err := row.Scan(
		&rule.ID,
		&rule.Name,
		&rule.GroupingKey,
		&rule.TimeWindowMinutes,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &rule, nil
}

// scanGroupingRuleRow scans a row from a Rows iterator into a GroupingRule.
func scanGroupingRuleRow(rows pgx.Rows) (*domain.GroupingRule, error) {
	var rule domain.GroupingRule

	err := rows.Scan(
		&rule.ID,
		&rule.Name,
		&rule.GroupingKey,
		&rule.TimeWindowMinutes,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &rule, nil
}
