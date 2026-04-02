package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	migrations "github.com/ClickHouse/ch-migrator-example/pkg"
	_ "github.com/ClickHouse/ch-migrator-example/pkg/migrations"
)

func TestDatabaseConnection(t *testing.T) {
	if sharedConn == nil {
		t.Fatal("Shared ClickHouse connection is nil")
	}
	err := sharedConn.Ping()
	if err != nil {
		t.Fatalf("Failed to ping ClickHouse from test: %v", err)
	}
	t.Log("Successfully pinged ClickHouse from test.")
}

func TestMigrate(t *testing.T) {
	testConn := GetTestDBConnection(t, "migrate_test_db")

	config := migrations.Config{
		ForceMergeTree: true,
		AllowMissing:   false,
		Revision:       0,
		DBPassword:     "Demo",
	}
	err := migrations.InitiateMigrations(testConn, config, migrations.EmbeddedMigrations)
	assert.NoError(t, err)

	t.Run("otel database created", func(t *testing.T) {
		var count int
		err := testConn.QueryRow("SELECT count() FROM system.databases WHERE name = 'otel'").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count, "otel database should exist")
	})

	t.Run("logs tables created", func(t *testing.T) {
		for _, table := range []string{"otel_logs", "otel_logs_json"} {
			var count int
			err := testConn.QueryRow("SELECT count() FROM system.tables WHERE database = 'otel' AND name = ?", table).Scan(&count)
			require.NoError(t, err)
			require.Equal(t, 1, count, "table %s should exist", table)
		}
	})

	t.Run("traces tables created", func(t *testing.T) {
		for _, table := range []string{"otel_traces", "otel_traces_json", "otel_traces_trace_id_ts"} {
			var count int
			err := testConn.QueryRow("SELECT count() FROM system.tables WHERE database = 'otel' AND name = ?", table).Scan(&count)
			require.NoError(t, err)
			require.Equal(t, 1, count, "table %s should exist", table)
		}
	})

	t.Run("traces materialized view created", func(t *testing.T) {
		var count int
		err := testConn.QueryRow("SELECT count() FROM system.tables WHERE database = 'otel' AND name = 'otel_traces_trace_id_ts_mv' AND engine = 'MaterializedView'").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count, "materialized view otel_traces_trace_id_ts_mv should exist")
	})

	t.Run("metrics tables created", func(t *testing.T) {
		for _, table := range []string{
			"otel_metrics_gauge",
			"otel_metrics_sum",
			"otel_metrics_histogram",
			"otel_metrics_exp_histogram",
			"otel_metrics_summary",
		} {
			var count int
			err := testConn.QueryRow("SELECT count() FROM system.tables WHERE database = 'otel' AND name = ?", table).Scan(&count)
			require.NoError(t, err)
			require.Equal(t, 1, count, "table %s should exist", table)
		}
	})

	t.Run("go migration added DeploymentEnvironment column", func(t *testing.T) {
		var count int
		err := testConn.QueryRow("SELECT count() FROM system.columns WHERE database = 'otel' AND table = 'otel_logs' AND name = 'DeploymentEnvironment'").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count, "DeploymentEnvironment column should exist on otel_logs")
	})

	t.Run("idempotent re-run succeeds", func(t *testing.T) {
		err := migrations.InitiateMigrations(testConn, config, migrations.EmbeddedMigrations)
		assert.NoError(t, err, "running migrations a second time should be idempotent")
	})
}
