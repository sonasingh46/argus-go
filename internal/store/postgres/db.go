// Package postgres provides PostgreSQL-based implementations of the store interfaces.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"argus-go/internal/config"
)

// DB wraps a PostgreSQL connection pool.
type DB struct {
	pool *pgxpool.Pool
}

// NewDB creates a new PostgreSQL connection pool.
func NewDB(ctx context.Context, cfg *config.PostgresConfig) (*DB, error) {
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s&pool_max_conns=%d",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.SSLMode,
		cfg.MaxOpenConns,
	)

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxOpenConns
	poolConfig.MinConns = cfg.MaxIdleConns
	poolConfig.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Pool returns the underlying connection pool.
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// Close closes the connection pool.
func (db *DB) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}

// RunMigrations creates the required database tables.
func (db *DB) RunMigrations(ctx context.Context) error {
	schema := `
		CREATE TABLE IF NOT EXISTS alerts (
			id VARCHAR(36) PRIMARY KEY,
			dedup_key VARCHAR(255) UNIQUE NOT NULL,
			event_manager_id VARCHAR(36) NOT NULL,
			summary TEXT NOT NULL,
			severity VARCHAR(20) NOT NULL,
			class VARCHAR(100) NOT NULL,
			type VARCHAR(20) NOT NULL,
			status VARCHAR(20) NOT NULL,
			parent_dedup_key VARCHAR(255),
			child_count INTEGER DEFAULT 0,
			resolve_requested BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
			resolved_at TIMESTAMP WITH TIME ZONE
		);

		CREATE INDEX IF NOT EXISTS idx_alerts_event_manager ON alerts(event_manager_id);
		CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status);
		CREATE INDEX IF NOT EXISTS idx_alerts_parent ON alerts(parent_dedup_key);
		CREATE INDEX IF NOT EXISTS idx_alerts_type ON alerts(type);

		CREATE TABLE IF NOT EXISTS event_managers (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			grouping_rule_id VARCHAR(36) NOT NULL,
			webhook_url TEXT,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL
		);

		CREATE TABLE IF NOT EXISTS grouping_rules (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			grouping_key VARCHAR(100) NOT NULL,
			time_window_minutes INTEGER NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL
		);
	`

	_, err := db.pool.Exec(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
