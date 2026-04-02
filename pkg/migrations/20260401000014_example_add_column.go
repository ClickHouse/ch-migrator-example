package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddDeploymentEnv, downAddDeploymentEnv)
}

// upAddDeploymentEnv demonstrates a simple Go migration that adds a column.
// This is equivalent to an ALTER TABLE in SQL but shows the Go migration pattern,
// which is useful when you need conditional logic or multi-step operations.
func upAddDeploymentEnv(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx,
		"ALTER TABLE otel.otel_logs ADD COLUMN IF NOT EXISTS DeploymentEnvironment LowCardinality(String) CODEC(ZSTD(1))")
	if err != nil {
		return fmt.Errorf("add DeploymentEnvironment column: %w", err)
	}
	return nil
}

func downAddDeploymentEnv(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx,
		"ALTER TABLE otel.otel_logs DROP COLUMN IF EXISTS DeploymentEnvironment")
	if err != nil {
		return fmt.Errorf("drop DeploymentEnvironment column: %w", err)
	}
	return nil
}
