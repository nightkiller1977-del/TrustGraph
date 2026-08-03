package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func RunMigrations(db *PostgresDB, migrationsDir string) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version   INTEGER PRIMARY KEY,
			dirty     BOOLEAN NOT NULL DEFAULT false,
			name      VARCHAR(255) NOT NULL,
			applied_at TIMESTAMPTZ DEFAULT now()
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) != 2 {
			continue
		}

		var version int
		if _, err := fmt.Sscanf(parts[0], "%d", &version); err != nil {
			continue
		}

		var applied int
		if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = $1", version).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if applied > 0 {
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.Name(), err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", entry.Name(), err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("exec %s: %w", entry.Name(), err)
		}

		if _, err := tx.Exec(
			"INSERT INTO schema_migrations (version, dirty, name) VALUES ($1, false, $2)",
			version, entry.Name(),
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("record %s: %w", entry.Name(), err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit %s: %w", entry.Name(), err)
		}
	}

	return nil
}
