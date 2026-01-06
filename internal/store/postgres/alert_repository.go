package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"argus-go/internal/domain"
)

// AlertRepository implements store.AlertRepository using PostgreSQL.
type AlertRepository struct {
	db *DB
}

// NewAlertRepository creates a new PostgreSQL-backed alert repository.
func NewAlertRepository(db *DB) *AlertRepository {
	return &AlertRepository{db: db}
}

// Create stores a new alert.
func (r *AlertRepository) Create(ctx context.Context, alert *domain.Alert) error {
	query := `
		INSERT INTO alerts (
			id, dedup_key, event_manager_id, summary, severity, class,
			type, status, parent_dedup_key, child_count, resolve_requested,
			created_at, updated_at, resolved_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err := r.db.pool.Exec(ctx, query,
		alert.ID,
		alert.DedupKey,
		alert.EventManagerID,
		alert.Summary,
		alert.Severity,
		alert.Class,
		alert.Type,
		alert.Status,
		nullableString(alert.ParentDedupKey),
		alert.ChildCount,
		alert.ResolveRequested,
		alert.CreatedAt,
		alert.UpdatedAt,
		alert.ResolvedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create alert: %w", err)
	}

	return nil
}

// Update modifies an existing alert.
func (r *AlertRepository) Update(ctx context.Context, alert *domain.Alert) error {
	query := `
		UPDATE alerts SET
			summary = $2,
			severity = $3,
			class = $4,
			status = $5,
			child_count = $6,
			resolve_requested = $7,
			updated_at = $8,
			resolved_at = $9
		WHERE id = $1
	`

	result, err := r.db.pool.Exec(ctx, query,
		alert.ID,
		alert.Summary,
		alert.Severity,
		alert.Class,
		alert.Status,
		alert.ChildCount,
		alert.ResolveRequested,
		alert.UpdatedAt,
		alert.ResolvedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update alert: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrAlertNotFound
	}

	return nil
}

// GetByID retrieves an alert by its database ID.
func (r *AlertRepository) GetByID(ctx context.Context, id string) (*domain.Alert, error) {
	return r.getOne(ctx, "id = $1", id)
}

// GetByDedupKey retrieves an alert by its deduplication key.
func (r *AlertRepository) GetByDedupKey(ctx context.Context, dedupKey string) (*domain.Alert, error) {
	return r.getOne(ctx, "dedup_key = $1", dedupKey)
}

// getOne retrieves a single alert matching the given condition.
func (r *AlertRepository) getOne(ctx context.Context, condition string, args ...interface{}) (*domain.Alert, error) {
	query := fmt.Sprintf(`
		SELECT id, dedup_key, event_manager_id, summary, severity, class,
			   type, status, parent_dedup_key, child_count, resolve_requested,
			   created_at, updated_at, resolved_at
		FROM alerts
		WHERE %s
	`, condition)

	row := r.db.pool.QueryRow(ctx, query, args...)

	alert, err := scanAlert(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAlertNotFound
		}
		return nil, fmt.Errorf("failed to get alert: %w", err)
	}

	return alert, nil
}

// List retrieves alerts matching the filter criteria.
func (r *AlertRepository) List(ctx context.Context, filter domain.AlertFilter) ([]*domain.Alert, error) {
	query := `
		SELECT id, dedup_key, event_manager_id, summary, severity, class,
			   type, status, parent_dedup_key, child_count, resolve_requested,
			   created_at, updated_at, resolved_at
		FROM alerts
		WHERE 1=1
	`
	args := []interface{}{}
	argNum := 1

	if filter.EventManagerID != "" {
		query += fmt.Sprintf(" AND event_manager_id = $%d", argNum)
		args = append(args, filter.EventManagerID)
		argNum++
	}

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, filter.Status)
		argNum++
	}

	if filter.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argNum)
		args = append(args, filter.Type)
		argNum++
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, filter.Limit)
		argNum++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list alerts: %w", err)
	}
	defer rows.Close()

	return scanAlerts(rows)
}

// GetChildrenByParent retrieves all child alerts for a given parent dedup key.
func (r *AlertRepository) GetChildrenByParent(ctx context.Context, parentDedupKey string) ([]*domain.Alert, error) {
	query := `
		SELECT id, dedup_key, event_manager_id, summary, severity, class,
			   type, status, parent_dedup_key, child_count, resolve_requested,
			   created_at, updated_at, resolved_at
		FROM alerts
		WHERE parent_dedup_key = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.pool.Query(ctx, query, parentDedupKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get children: %w", err)
	}
	defer rows.Close()

	return scanAlerts(rows)
}

// CountActiveChildren returns the count of active child alerts for a parent.
func (r *AlertRepository) CountActiveChildren(ctx context.Context, parentDedupKey string) (int, error) {
	query := `
		SELECT COUNT(*) FROM alerts
		WHERE parent_dedup_key = $1 AND status = 'active'
	`

	var count int
	err := r.db.pool.QueryRow(ctx, query, parentDedupKey).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active children: %w", err)
	}

	return count, nil
}

// scanAlert scans a single row into an Alert.
func scanAlert(row pgx.Row) (*domain.Alert, error) {
	var alert domain.Alert
	var parentDedupKey *string

	err := row.Scan(
		&alert.ID,
		&alert.DedupKey,
		&alert.EventManagerID,
		&alert.Summary,
		&alert.Severity,
		&alert.Class,
		&alert.Type,
		&alert.Status,
		&parentDedupKey,
		&alert.ChildCount,
		&alert.ResolveRequested,
		&alert.CreatedAt,
		&alert.UpdatedAt,
		&alert.ResolvedAt,
	)

	if err != nil {
		return nil, err
	}

	if parentDedupKey != nil {
		alert.ParentDedupKey = *parentDedupKey
	}

	return &alert, nil
}

// scanAlerts scans multiple rows into a slice of Alerts.
func scanAlerts(rows pgx.Rows) ([]*domain.Alert, error) {
	var alerts []*domain.Alert

	for rows.Next() {
		var alert domain.Alert
		var parentDedupKey *string

		err := rows.Scan(
			&alert.ID,
			&alert.DedupKey,
			&alert.EventManagerID,
			&alert.Summary,
			&alert.Severity,
			&alert.Class,
			&alert.Type,
			&alert.Status,
			&parentDedupKey,
			&alert.ChildCount,
			&alert.ResolveRequested,
			&alert.CreatedAt,
			&alert.UpdatedAt,
			&alert.ResolvedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan alert: %w", err)
		}

		if parentDedupKey != nil {
			alert.ParentDedupKey = *parentDedupKey
		}

		alerts = append(alerts, &alert)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating alerts: %w", err)
	}

	return alerts, nil
}

// nullableString returns nil if the string is empty, otherwise returns a pointer to it.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
