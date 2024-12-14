package migration

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestMigrator(t *testing.T) {
	// Skip if running short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test Up migration
	t.Run("Up", func(t *testing.T) {
		var err error

		// Setup
		db := setupTestDB(t)
		defer db.Close()

		migrationDir, cleanupMigrations := createTestMigrations(t)
		defer cleanupMigrations()

		// Change working directory to the migration directory
		originalWd, _ := os.Getwd()

		err = os.Chdir(migrationDir)
		require.NoError(t, err)

		defer func() {
			err = os.Chdir(originalWd)
			require.NoError(t, err)
		}()

		// Create Migrator
		migrator := NewMigrator(db, migrationDir)
		require.NotNil(t, migrator)

		err = migrator.Up(context.Background())
		require.NoError(t, err)

		defer cleanTestMigrations(t, db)

		// Verify migration
		var tableExists bool
		err = db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'test_table')").Scan(&tableExists)
		require.NoError(t, err)
		require.True(t, tableExists)
	})

	// Test idempotency of Up
	t.Run("Up Idempotency", func(t *testing.T) {
		var err error

		// Setup
		db := setupTestDB(t)
		defer db.Close()

		migrationDir, cleanupMigrations := createTestMigrations(t)
		defer cleanupMigrations()

		// Change working directory to the migration directory
		originalWd, _ := os.Getwd()

		err = os.Chdir(migrationDir)
		require.NoError(t, err)

		defer func() {
			err = os.Chdir(originalWd)
			require.NoError(t, err)
		}()

		// Create Migrator
		migrator := NewMigrator(db, migrationDir)
		require.NotNil(t, migrator)

		err = migrator.Up(context.Background())
		require.NoError(t, err)

		defer cleanTestMigrations(t, db)

		err = migrator.Up(context.Background())
		require.NoError(t, err)
	})

	// Test custom logger
	t.Run("Custom Logger", func(t *testing.T) {
		var err error

		// Setup
		db := setupTestDB(t)
		defer db.Close()

		migrationDir, cleanupMigrations := createTestMigrations(t)
		defer cleanupMigrations()

		// Change working directory to the migration directory
		originalWd, _ := os.Getwd()

		err = os.Chdir(migrationDir)
		require.NoError(t, err)

		defer func() {
			err = os.Chdir(originalWd)
			require.NoError(t, err)
		}()

		// Create Migrator
		migrator := NewMigrator(db, migrationDir)
		require.NotNil(t, migrator)

		defer cleanTestMigrations(t, db)

		customLogger := log.New(os.Stdout, "TEST: ", log.Ldate|log.Ltime|log.Lshortfile)
		migrator.WithCustomLogger(customLogger)
		err = migrator.Up(context.Background())
		require.NoError(t, err)
	})
}

func TestMigratorErrors(t *testing.T) {
	// Skip if running short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Invalid pattern", func(t *testing.T) {
		// Setup
		db := setupTestDB(t)
		defer db.Close()

		// Create a migrator with an invalid pattern
		invalidPath := string([]byte{0}) // null byte is invalid in file paths
		migrator := NewMigrator(db, invalidPath)
		migrator.globFunc = func(_ string) ([]string, error) {
			return nil, errors.New("syntax error in pattern")
		}

		err := migrator.Up(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "syntax error in pattern")
	})

	t.Run("Insufficient permissions", func(t *testing.T) {
		// Setup
		db := setupTestDB(t)
		defer db.Close()

		// Create a directory with no read permissions
		noPermDir, err := os.MkdirTemp("", "no_perm_migrations")
		require.NoError(t, err)
		defer os.RemoveAll(noPermDir)

		err = os.Chmod(noPermDir, 0000) // Remove all permissions
		require.NoError(t, err)

		migrator := NewMigrator(db, noPermDir)
		migrator.globFunc = func(_ string) ([]string, error) {
			return nil, errors.New("permission denied")
		}

		err = migrator.Up(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "permission denied")
	})

	t.Run("Invalid connection", func(t *testing.T) {
		var err error

		// Create a migrator with an invalid database connection
		invalidDB, err := sql.Open("postgres", "postgres://invalid:invalid@localhost:12343/invalid?sslmode=disable")
		require.NoError(t, err)

		migrationDir, cleanupMigrations := createTestMigrations(t)
		defer cleanupMigrations()

		// Change working directory to the migration directory
		originalWd, _ := os.Getwd()

		err = os.Chdir(migrationDir)
		require.NoError(t, err)

		defer func() {
			err = os.Chdir(originalWd)
			require.NoError(t, err)
		}()

		migrator := NewMigrator(invalidDB, migrationDir)
		err = migrator.Up(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "connect: connection refused")
	})

	t.Run("Database error", func(t *testing.T) {
		var err error

		// Setup
		db := setupTestDB(t)

		migrationDir, cleanupMigrations := createTestMigrations(t)
		defer cleanupMigrations()

		// Change working directory to the migration directory
		originalWd, _ := os.Getwd()

		err = os.Chdir(migrationDir)
		require.NoError(t, err)

		defer func() {
			err = os.Chdir(originalWd)
			require.NoError(t, err)
		}()

		migrator := NewMigrator(db, migrationDir)

		// Close the database connection
		db.Close()

		err = migrator.Up(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "sql: database is closed")
	})
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// if running using docker compose:
	// db, err := sql.Open("postgres", "user=postgres password=postgres host=postgres port=5432 dbname=postgres sslmode=disable")
	// if running outside the container:
	// db, err := sql.Open("postgres", "user=postgres password=postgres host=localhost port=5433 dbname=postgres sslmode=disable")
	db, err := sql.Open("postgres", "user=postgres password=postgres host=postgres port=5432 dbname=postgres sslmode=disable")
	require.NoError(t, err)

	err = db.Ping()
	require.NoError(t, err)

	return db
}

func createTestMigrations(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "test_migrations")
	require.NoError(t, err)

	upSQL := `CREATE TABLE test_table (id SERIAL PRIMARY KEY, name TEXT);`
	err = os.WriteFile(filepath.Join(dir, "001_create_test_table.up.sql"), []byte(upSQL), 0644)
	require.NoError(t, err)

	return dir, func() {
		os.RemoveAll(dir)
	}
}

func cleanTestMigrations(t *testing.T, db *sql.DB) {
	t.Helper()

	_, err := db.Exec("DROP TABLE IF EXISTS test_table")
	require.NoError(t, err)

	_, err = db.Exec("TRUNCATE TABLE migrations")
	require.NoError(t, err)
}
