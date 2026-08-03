package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// PostgresDB wraps a sql.DB connection pool
type PostgresDB struct {
	*sql.DB
}

// NewPostgres creates a new PostgreSQL connection pool
func NewPostgres(ctx context.Context, dsn string) (*PostgresDB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * 60) // 5 minutes

	return &PostgresDB{db}, nil
}

// Close closes the database connection pool
func (p *PostgresDB) Close() error {
	return p.DB.Close()
}
