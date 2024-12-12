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

	_ "github.com/lib/pq" // Or your database driver
)

type Operation string

const (
	OperationUp   Operation = "up"
	OperationDown Operation = "down"
)

type Migrator struct {
	db            *sql.DB
	logger        *log.Logger
	migrationPath string
}

func NewMigrator(db *sql.DB, migrationPath string) *Migrator {
	return &Migrator{
		db:            db,
		logger:        log.Default(),
		migrationPath: migrationPath,
	}
}

func (r *Migrator) WithCustomLogger(logger *log.Logger) *Migrator {
	r.logger = logger
	return r
}

func (m *Migrator) Up(ctx context.Context) error {
	fmt.Println("starting migration")

	m.createMigrationTable(ctx)

	files, err := filepath.Glob(fmt.Sprintf("%s/*_up.sql", m.migrationPath))
	if err != nil {
		return err
	}

	sort.Strings(files)

	fmt.Printf("Found %d migrations\n", len(files))

	for _, file := range files {
		fmt.Println(file)
		exists, err := m.exists(ctx, filepath.Base(file))
		if err != nil {
			return err
		}

		if exists {
			m.logger.Println("SKIP: Migration already applied:", filepath.Base(file))
			continue
		}

		err = m.applyMigration(ctx, file, OperationUp)
		if err != nil {
			return err
		}

		m.logger.Printf("Migration applied: %s\n", filepath.Base(file))
	}

	return nil
}

func (m *Migrator) Down(ctx context.Context) error {
	m.createMigrationTable(ctx)

	files, err := filepath.Glob(fmt.Sprintf("%s/*_down.sql", m.migrationPath))
	if err != nil {
		return err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j]
	})

	for _, file := range files {
		exists, err := m.exists(ctx, filepath.Base(file))
		if err != nil {
			return err
		}

		if !exists {
			m.logger.Println("SKIP: Migration already reversed:", filepath.Base(file))
			continue
		}

		err = m.applyMigration(ctx, file, OperationDown)
		if err != nil {
			return err
		}

		m.logger.Printf("Migration applied: %s\n", filepath.Base(file))
	}

	return nil
}

func (m *Migrator) createMigrationTable(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS migrations (
		id SERIAL NOT NULL,
		name VARCHAR(255) NOT NULL,
		created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (id)
	);`)
	if err != nil {
		return err
	}

	return nil
}

func (m *Migrator) exists(ctx context.Context, name string) (bool, error) {
	var count int
	err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM migrations WHERE name = $1", name).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (m *Migrator) applyMigration(ctx context.Context, file string, operation Operation) error {
	fileIO, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fileIO.Close()

	content, err := io.ReadAll(fileIO)
	if err != nil {
		return err
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, string(content))
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	if operation == OperationUp {
		_, err = tx.ExecContext(ctx, "INSERT INTO migrations (name) VALUES ($1)", filepath.Base(file))
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	} else {
		_, err = tx.ExecContext(ctx, "DELETE FROM migrations WHERE name = $1", filepath.Base(file))
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
