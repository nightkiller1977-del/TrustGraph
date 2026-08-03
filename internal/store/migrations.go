package store

import (
	"database/sql"
	"fmt"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
)

// RunMigrations runs all migration files in the migrations directory
func RunMigrations(db *PostgresDB, migrationsDir string) error {
	// Create migrations tracking table
	createMigrationsTableSQL := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			dirty BOOLEAN NOT NULL DEFAULT false,
			name VARCHAR(255) NOT NULL
		)
	`
	if _, err := db.Exec(createMigrationsTableSQL); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Read migration files
	files, err := ioutil.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Sort migration files by name
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	// Run each migration file
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		// Extract version number from filename (e.g., "001_init_schema.sql" -> 1)
		parts := strings.SplitN(file.Name(), "_", 2)
		if len(parts) != 2 {
			continue
		}

		versionStr := parts[0]
		var version int
		if _, err := fmt.Sscanf(versionStr, "%d", &version); err != nil {
			continue
		}

		// Check if migration already applied
		var applied int
		err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = $1", version).Scan(&applied)
		if err != nil {
			return fmt.Errorf("failed to check migration status: %w", err)
		}

		if applied > 0 {
			continue // Already applied
		}

		// Read and execute migration
		filePath := filepath.Join(migrationsDir, file.Name())
		content, err := ioutil.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", filePath, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", file.Name(), err)
		}

		if _, err := tx.Exec(
			"INSERT INTO schema_migrations (version, dirty, name) VALUES ($1, false, $2)",
			version, file.Name(),
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", file.Name(), err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", file.Name(), err)
		}
	}

	return nil
}
