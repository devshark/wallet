package migration

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/devshark/wallet/api"
	_ "github.com/lib/pq" // Or your database driver
)

type GlobFunc func(pattern string) (matches []string, err error)

const (
	createMigration = `CREATE TABLE IF NOT EXISTS migrations (
		id SERIAL NOT NULL,
		name VARCHAR(255) NOT NULL,
		created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (id)
	);`
)

type Migrator struct {
	db            *sql.DB
	logger        *log.Logger
	migrationPath string
	globFunc      GlobFunc // makes testing easier
}

func NewMigrator(db *sql.DB, migrationPath string) *Migrator {
	return &Migrator{
		db:            db,
		logger:        log.Default(),
		migrationPath: migrationPath,
		globFunc:      filepath.Glob,
	}
}

func (m *Migrator) WithCustomLogger(logger *log.Logger) *Migrator {
	m.logger = logger

	return m
}

func (m *Migrator) Up(ctx context.Context) error {
	if err := m.createMigrationTable(ctx); err != nil {
		return formatUnknownError(err)
	}

	files, err := m.globFunc(fmt.Sprintf("%s/*.up.sql", m.migrationPath))
	if err != nil {
		return formatUnknownError(err)
	}

	sort.Strings(files)

	m.logger.Printf("Found %d migrations\n", len(files))

	for _, file := range files {
		exists, err := m.exists(ctx, filepath.Base(file))
		if err != nil {
			return formatUnknownError(err)
		}

		if exists {
			m.logger.Println("SKIP: Migration already applied:", filepath.Base(file))

			continue
		}

		err = m.applyMigration(ctx, file)
		if err != nil {
			return formatUnknownError(err)
		}

		m.logger.Printf("Migration applied: %s\n", filepath.Base(file))
	}

	return nil
}

func formatUnknownError(err error) error {
	return fmt.Errorf("%w: %w", api.ErrUnhandledDatabaseError, err)
}

func (m *Migrator) createMigrationTable(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, createMigration)
	if err != nil {
		return formatUnknownError(err)
	}

	return nil
}

func (m *Migrator) exists(ctx context.Context, name string) (bool, error) {
	var count int

	err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM migrations WHERE name = $1", name).Scan(&count)
	if err != nil {
		return false, formatUnknownError(err)
	}

	return count > 0, nil
}

func (m *Migrator) applyMigration(ctx context.Context, file string) error {
	fileIO, err := os.Open(file)
	if err != nil {
		return formatUnknownError(err)
	}
	defer fileIO.Close()

	content, err := io.ReadAll(fileIO)
	if err != nil {
		return formatUnknownError(err)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return formatUnknownError(err)
	}

	_, err = tx.ExecContext(ctx, string(content))
	if err != nil {
		_ = tx.Rollback()

		return formatUnknownError(err)
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO migrations (name) VALUES ($1)", filepath.Base(file))
	if err != nil {
		_ = tx.Rollback()

		return formatUnknownError(err)
	}

	err = tx.Commit()
	if err != nil {
		return formatUnknownError(err)
	}

	return nil
}
