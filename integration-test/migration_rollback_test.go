package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ClickHouse/ch-migrator-example/integration-test/test_migrations/test_failing_migration"
	migrations "github.com/ClickHouse/ch-migrator-example/pkg"
)

// TestMigratorRollbackFailure tests that the migrator properly handles failing migrations
// by rolling back only the failing migration while keeping successful ones
func TestMigratorRollbackFailure(t *testing.T) {
	// Get an isolated database connection for this test
	testConn := GetTestDBConnection(t, "rollback_test_db")

	// Call InitiateMigrations with our test config
	config := migrations.Config{
		AllowMissing: false,
		Revision:     0, // latest
	}

	// The InitiateMigrations will panic if the migration fails, so we need to recover
	var migrationErr error
	defer func() {
		if r := recover(); r != nil {
			// Store the panic error for later assertions
			if err, ok := r.(error); ok {
				migrationErr = err
				t.Logf("Recovered from panic during migration: %v", r)
			} else {
				t.Logf("Recovered from panic during migration (non-error): %v", r)
			}
		}
	}()

	// This will panic due to the failing migration
	migrationErr = migrations.InitiateMigrations(testConn, config, test_failing_migration.TestFailingMigrations)

	// If we get here without a panic, that's a problem
	if migrationErr == nil {
		t.Fatal("Expected migration to fail but no error was encountered")
	}

	// Verify that first migration was applied (test_db exists)
	var exists1 int
	err := testConn.QueryRow("SELECT count(*) FROM system.databases WHERE name = 'test_migration_db'").Scan(&exists1)
	assert.NoError(t, err)
	assert.Equal(t, 1, exists1, "Database test_migration_db should exist")

	// Verify that first migration's table exists
	var exists2 int
	err = testConn.QueryRow("SELECT count(*) FROM system.tables WHERE database = 'test_migration_db' AND name = 'test_table1'").Scan(&exists2)
	assert.NoError(t, err)
	assert.Equal(t, 1, exists2, "Table test_table1 should exist")

	// Verify that second migration was rolled back (test_table2 doesn't exist)
	var exists3 int
	err = testConn.QueryRow("SELECT count(*) FROM system.tables WHERE database = 'test_migration_db' AND name = 'test_table2'").Scan(&exists3)
	assert.NoError(t, err)
	assert.Equal(t, 0, exists3, "Table test_table2 should not exist as migration should have been rolled back")

	// Verify that third migration was not applied (test_table3 doesn't exist)
	var exists4 int
	err = testConn.QueryRow("SELECT count(*) FROM system.tables WHERE database = 'test_migration_db' AND name = 'test_table3'").Scan(&exists4)
	assert.NoError(t, err)
	assert.Equal(t, 0, exists4, "Table test_table3 should not exist as migration was never executed")

	// Verify the goose version table contains only the first migration
	var versions []int64
	rows, err := testConn.Query("SELECT version_id FROM goose_db_version WHERE is_applied = 1 ORDER BY version_id")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var version int64
		err := rows.Scan(&version)
		require.NoError(t, err)
		versions = append(versions, version)
	}

	t.Logf("Applied migration versions: %v", versions)

	// Only the first migration should be marked as applied
	assert.Contains(t, versions, int64(20250520000000), "Version 20250520000000 should be in the goose version table")
	assert.NotContains(t, versions, int64(20250520000001), "Version 20250520000001 should not be in the goose version table")
	assert.NotContains(t, versions, int64(20250520000002), "Version 20250520000002 should not be in the goose version table")
}
