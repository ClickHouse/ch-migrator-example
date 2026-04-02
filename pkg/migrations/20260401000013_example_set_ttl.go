package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog/log"
)

func init() {
	goose.AddMigrationContext(upSetMetricsTTL, downSetMetricsTTL)
}

// upSetMetricsTTL demonstrates a Go migration that dynamically alters TTL
// on all metrics tables matching a pattern. This is useful when you want to
// apply the same change across multiple tables without writing individual
// SQL migrations for each one.
func upSetMetricsTTL(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx,
		"SELECT name FROM system.tables WHERE database = 'otel' AND name LIKE 'otel_metrics_%' AND engine != 'View'")
	if err != nil {
		return fmt.Errorf("query metrics tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan table name: %w", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tables: %w", err)
	}

	for _, table := range tables {
		stmt := fmt.Sprintf("ALTER TABLE otel.`%s` MODIFY TTL toDateTime(TimeUnix) + toIntervalDay(90)", table)
		log.Info().Str("table", table).Msg("setting metrics TTL to 90 days")
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("alter TTL on %s: %w", table, err)
		}
	}

	return nil
}

func downSetMetricsTTL(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx,
		"SELECT name FROM system.tables WHERE database = 'otel' AND name LIKE 'otel_metrics_%' AND engine != 'View'")
	if err != nil {
		return fmt.Errorf("query metrics tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan table name: %w", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tables: %w", err)
	}

	for _, table := range tables {
		stmt := fmt.Sprintf("ALTER TABLE otel.`%s` MODIFY TTL toDateTime(TimeUnix) + toIntervalDay(180)", table)
		log.Info().Str("table", table).Msg("reverting metrics TTL to 180 days")
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("revert TTL on %s: %w", table, err)
		}
	}

	return nil
}
