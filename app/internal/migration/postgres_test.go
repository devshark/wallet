package migration_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/devshark/wallet/app/internal/migration"
	pgTesting "github.com/devshark/wallet/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/lib/pq"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	if testing.Short() {
		t.Skip("Skipping test in short mode, as it requires a database")
	}

	connectionString, cleanup := pgTesting.SetupTestDB(t)
	// db, err := sql.Open("postgres", "user=postgres password=postgres host=localhost port=5433 dbname=postgres sslmode=disable")
	db, err := sql.Open("postgres", connectionString)
	// db, err := sql.Open("postgres", "postgres://user:password@localhost:5432/testdb?sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	return db, cleanup
}

func createMigrationFiles(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "migrations")
	require.NoError(t, err)

	t.Logf("Created temporary directory: %s", tempDir)

	// Create test migration files
	upContent := []byte("CREATE TABLE public.test_table (id SERIAL PRIMARY KEY, name TEXT);")
	downContent := []byte("DROP TABLE IF EXISTS public.test_table;")

	// os.CreateTemp("migrations", "")

	err = os.WriteFile(filepath.Join(tempDir, "001_create_test_table_up.sql"), upContent, 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "001_create_test_table_down.sql"), downContent, 0644)
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

func TestMigrator(t *testing.T) {
	db, _ := setupTestDB(t)
	// defer cleanupDb()

	defer db.Close()

	migrationDir, cleanup := createMigrationFiles(t)
	defer cleanup()

	// Change working directory to the migration directory
	originalWd, _ := os.Getwd()

	t.Logf("Current working directory: %s", originalWd)

	err := os.Chdir(migrationDir)
	require.NoError(t, err)
	// defer os.Chdir(originalWd)
	originalWd, _ = os.Getwd()
	t.Logf("New working directory: %s", originalWd)
	entries, err := os.ReadDir(originalWd)
	require.NoError(t, err)
	t.Logf("Entries: %v", entries)

	ctx := context.Background()

	t.Run("NewMigrator", func(t *testing.T) {
		migrator := migration.NewMigrator(db, migrationDir)
		assert.NotNil(t, migrator)
	})

	t.Run("Up", func(t *testing.T) {
		migrator := migration.NewMigrator(db, migrationDir)
		err := migrator.Up(ctx)
		assert.NoError(t, err)

		// Verify that the table was created
		var tableExists bool
		err = db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'test_table')").Scan(&tableExists)
		assert.NoError(t, err)
		assert.True(t, tableExists)
	})

	t.Run("Down", func(t *testing.T) {
		migrator := migration.NewMigrator(db, migrationDir)
		err := migrator.Down(ctx)
		assert.NoError(t, err)

		// Verify that the table was dropped
		var tableExists bool
		err = db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'test_table')").Scan(&tableExists)
		assert.NoError(t, err)
		assert.False(t, tableExists)
	})

	t.Run("Up and Down Idempotency", func(t *testing.T) {
		migrator := migration.NewMigrator(db, migrationDir)

		// Run Up twice
		err := migrator.Up(ctx)
		assert.NoError(t, err)
		err = migrator.Up(ctx)
		assert.NoError(t, err)

		// Verify that the table still exists
		var tableExists bool
		err = db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'test_table')").Scan(&tableExists)
		assert.NoError(t, err)
		assert.True(t, tableExists)

		// Run Down twice
		err = migrator.Down(ctx)
		assert.NoError(t, err)
		err = migrator.Down(ctx)
		assert.NoError(t, err)

		// Verify that the table no longer exists
		err = db.QueryRow("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'test_table')").Scan(&tableExists)
		assert.NoError(t, err)
		assert.False(t, tableExists)
	})
}
