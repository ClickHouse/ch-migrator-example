# ch-migrator-example User Guide

A ClickHouse migration tool that manages schema evolution using [goose](https://github.com/pressly/goose), with full [OpenTelemetry](https://opentelemetry.io/) schema support out of the box.

---

## Table of contents

- [Overview](#overview)
- [Getting started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Build](#build)
  - [Run your first migration](#run-your-first-migration)
- [What gets created](#what-gets-created)
- [Configuration reference](#configuration-reference)
  - [CLI flags](#cli-flags)
  - [Environment variables](#environment-variables)
  - [YAML config file](#yaml-config-file)
- [Writing migrations](#writing-migrations)
  - [SQL migrations](#sql-migrations)
  - [Go migrations](#go-migrations)
  - [Migration ordering](#migration-ordering)
- [Targeting a specific revision](#targeting-a-specific-revision)
- [Using as a Go library](#using-as-a-go-library)
- [Testing](#testing)
- [Deployment patterns](#deployment-patterns)
- [Troubleshooting](#troubleshooting)

---

## Overview

ch-migrator-example is a **reference implementation** of a ClickHouse schema migration tool. It is intended to be forked, adapted, and extended for your own schemas — not used as-is in production without modification. The bundled OpenTelemetry migrations serve as a working example of the patterns the framework supports.

The framework handles:

- **Ordered execution** — Migrations run in timestamp order with tracking via a `goose_db_version` table
- **Rollback** — Every migration has an `up` and `down` path
- **SQL and Go migrations** — Simple DDL in SQL files, complex logic in Go

The bundled migrations create the standard OpenTelemetry schema for logs, traces, and metrics — the same schema used by the [OTel Collector ClickHouse exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/clickhouseexporter). Replace or extend these with your own schemas to build a migrator tailored to your application.

---

## Getting started

### Prerequisites

- **Go 1.25+**
- **A running ClickHouse instance** (local, Docker, or ClickHouse Cloud)
- **Make** (optional, for convenience targets)

### Build

```bash
git clone https://github.com/ClickHouse/ch-migrator-example.git
cd ch-migrator-example/migrator
make build
```

This produces `bin/ch-migrator-example`.

### Run your first migration

**Local ClickHouse (no TLS):**

```bash
./bin/ch-migrator-example \
  --dbAddr localhost \
  --dbPort 9000 \
  --enableTLS=false \
  --dbUsername default \
  --dbPassword ''
```

**ClickHouse Cloud (HTTPS):**

```bash
./bin/ch-migrator-example \
  --dbAddr abc123.us-east-1.aws.clickhouse.cloud \
  --dbPort 8443 \
  --useHTTP \
  --dbUsername default \
  --dbPassword 'your-password'
```

On success you'll see each migration applied:

```
{"level":"debug","revision":0,"time":"2026-04-02T12:16:07Z","message":"migration from revision"}
{"level":"info","time":"2026-04-02T12:16:07Z","message":"migrating to latest"}
{"level":"info","migration":"20260401000001_create_otel_database.sql","duration":"76.29ms","time":"2026-04-02T12:16:07Z","message":"migration applied"}
...
{"level":"info","time":"2026-04-02T12:16:08Z","message":"goose: successfully migrated database to version: 20260401000014"}
{"level":"info","time":"2026-04-02T12:16:08Z","message":"migration completed successfully"}
```

Running the migrator again is safe — it's idempotent. Already-applied migrations are skipped.

---

## What gets created

After running all bundled migrations, your ClickHouse instance will have:

### Database

| Database | Purpose |
|----------|---------|
| `otel` | All OpenTelemetry tables |

### Migration tracking

Goose automatically creates a `goose_db_version` table in the default database to track which migrations have been applied:

```sql
CREATE TABLE default.goose_db_version (
    version_id Int64,
    is_applied UInt8,
    date Date DEFAULT now(),
    tstamp DateTime DEFAULT now()
) ENGINE = MergeTree()
ORDER BY date
```

This table is managed by goose — do not modify it manually.

### Tables

| Table | Type | Description |
|-------|------|-------------|
| `otel.otel_logs` | Logs | Log records with Map-typed attributes |
| `otel.otel_logs_json` | Logs | Log records with JSON-typed attributes |
| `otel.otel_traces` | Traces | Span data with Map-typed attributes |
| `otel.otel_traces_json` | Traces | Span data with JSON-typed attributes |
| `otel.otel_traces_trace_id_ts` | Traces | Trace ID to timestamp lookup table |
| `otel.otel_traces_trace_id_ts_mv` | Traces | Materialized view populating the lookup table |
| `otel.otel_metrics_gauge` | Metrics | Gauge metric data points |
| `otel.otel_metrics_sum` | Metrics | Sum (counter) metric data points |
| `otel.otel_metrics_histogram` | Metrics | Histogram metric data points |
| `otel.otel_metrics_exp_histogram` | Metrics | Exponential histogram data points |
| `otel.otel_metrics_summary` | Metrics | Summary metric data points |
---

## Configuration reference

Configuration is resolved in this priority order (highest first):

1. Environment variables
2. CLI flags
3. YAML config file

### CLI flags

```bash
./bin/ch-migrator-example \
  --dbAddr localhost \          # ClickHouse host (required)
  --dbPort 9000 \               # ClickHouse native protocol port (required)
  --dbUsername default \# User to run migrations as
  --dbPassword '' \    # Password
  --enableTLS=false \           # Disable TLS (default: true)
  --insecureSkipTLSVerify=true \# Skip certificate verification (default: true)
  --useHTTP \                   # Use HTTP protocol (for HTTPS on port 8443)
  --allowMissing \              # Allow out-of-order migration application
  --revision 20260401000007 \   # Migrate to a specific revision (default: latest)
  --config config.yaml          # Load settings from a YAML file
```

### Environment variables

Every flag can be set via environment variable. Viper resolves them case-insensitively:

```bash
export DBADDR=localhost
export DBPORT=9000
export DBUSERNAME=default
export DBPASSWORD=secret
export ENABLETLS=false

./bin/ch-migrator-example
```

### YAML config file

```yaml
dbAddr: localhost
dbPort: "9000"
enableTLS: false
DBUSERNAME: default
DBPASSWORD: secret
allowMissing: false
```

```bash
./bin/ch-migrator-example --config config.yaml
```

---

## Writing migrations

All migration files go in `pkg/migrations/`. The migrator embeds this directory at compile time.

### SQL migrations

Create a new migration using the [goose CLI](https://pressly.github.io/goose/installation/) via Make:

```bash
make new-migration NAME=add_user_events
```

This creates a timestamped `.sql` file in `pkg/migrations/`. Fill in the goose annotations:

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS otel.user_events (
    Timestamp DateTime64(9) CODEC(Delta(8), ZSTD(1)),
    UserId String CODEC(ZSTD(1)),
    EventType LowCardinality(String) CODEC(ZSTD(1)),
    Payload String CODEC(ZSTD(1))
) ENGINE = MergeTree()
PARTITION BY toDate(Timestamp)
ORDER BY (UserId, Timestamp)
TTL toDateTime(Timestamp) + toIntervalDay(90)
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- +goose Down
DROP TABLE IF EXISTS otel.user_events;
```

### Go migrations

Use Go when you need logic that SQL can't express — iterating tables, conditional changes, or reading system tables.

```bash
make new-go-migration NAME=set_metrics_ttl
```

Then fill in the generated file in `pkg/migrations/`:

```go
package migrations

import (
    "context"
    "database/sql"
    "fmt"

    "github.com/pressly/goose/v3"
    "github.com/rs/zerolog/log"
)

func init() {
    goose.AddMigrationContext(upMyChange, downMyChange)
}

func upMyChange(ctx context.Context, tx *sql.Tx) error {
    // Example: set TTL on all tables matching a pattern
    rows, err := tx.QueryContext(ctx,
        "SELECT name FROM system.tables WHERE database = 'otel' AND name LIKE 'otel_metrics_%'")
    if err != nil {
        return err
    }
    defer func() { _ = rows.Close() }()

    for rows.Next() {
        var name string
        if err := rows.Scan(&name); err != nil {
            return err
        }
        stmt := fmt.Sprintf("ALTER TABLE otel.`%s` MODIFY TTL toDateTime(TimeUnix) + toIntervalDay(90)", name)
        log.Info().Str("table", name).Msg("updating TTL")
        if _, err := tx.ExecContext(ctx, stmt); err != nil {
            return fmt.Errorf("alter %s: %w", name, err)
        }
    }
    return rows.Err()
}

func downMyChange(ctx context.Context, tx *sql.Tx) error {
    // Reverse the change, or return an error if irreversible:
    return fmt.Errorf("irreversible migration: cannot restore original TTLs")
}
```

Key rules for Go migrations:
- The file must be in `pkg/migrations/` with `package migrations`
- Call `goose.AddMigrationContext(up, down)` in `init()`
- Always use `tx.ExecContext(ctx, ...)` — not `tx.Exec()` — to respect context cancellation
- Always close rows and check `rows.Err()`

### Migration ordering

Goose runs migrations in lexicographic order by filename. The timestamp prefix (e.g., `20260401000001`) ensures chronological ordering. When adding migrations:

- Use `$(date +%Y%m%d%H%M%S)` to generate unique timestamps
- Never reuse or modify a timestamp that has already been applied to a database
- If two developers create migrations concurrently, `--allowMissing` lets them be applied out of order

---

## Targeting a specific revision

By default, the migrator applies all pending migrations. To migrate to a specific version:

```bash
# Migrate UP to revision 20260401000007 (traces MV)
./bin/ch-migrator-example --dbAddr localhost --dbPort 9000 \
  --enableTLS=false \
  --revision 20260401000007

# Migrate DOWN to revision 20260401000002 (only logs table remains)
./bin/ch-migrator-example --dbAddr localhost --dbPort 9000 \
  --enableTLS=false \
  --revision 20260401000002
```

The migrator automatically determines the direction (up or down) by comparing the current database version to the target revision.

---

## Using as a Go library

The migration engine can be imported into your own Go application:

```go
package main

import (
    "database/sql"
    "log"

    clickhouse "github.com/ClickHouse/clickhouse-go/v2"
    migrations "github.com/ClickHouse/ch-migrator-example/pkg"

    // Register the bundled Go migrations.
    // Omit this import if you only want SQL migrations or your own Go migrations.
    _ "github.com/ClickHouse/ch-migrator-example/pkg/migrations"
)

func main() {
    db := clickhouse.OpenDB(&clickhouse.Options{
        Addr: []string{"localhost:9000"},
        Auth: clickhouse.Auth{Username: "default"},
    })

    cfg := migrations.Config{
        Revision: migrations.RevisionLatest,
    }

    if err := migrations.InitiateMigrations(db, cfg, migrations.EmbeddedMigrations); err != nil {
        log.Fatal(err)
    }
}
```

You can also supply your own `fs.FS` with custom migrations instead of `EmbeddedMigrations`:

```go
//go:embed my_migrations
var myMigrations embed.FS

err := migrations.InitiateMigrations(db, cfg, myMigrations)
```

### API surface

| Symbol | Type | Description |
|--------|------|-------------|
| `migrations.Config` | struct | Database connection and migration options |
| `migrations.InitiateMigrations(db, cfg, fs)` | func | Run migrations against a `*sql.DB` |
| `migrations.EmbeddedMigrations` | `embed.FS` | The bundled OTel migration files |
| `migrations.RevisionLatest` | `const int64` | Sentinel value `0` meaning "migrate to latest" |

---

## Testing

### Unit tests

```bash
make test
```

Runs unit tests and regression tests for known bugs.

### Integration tests

```bash
make integration-test
```

Requires a local container runtime (Docker, OrbStack, Podman). Uses [testcontainers-go](https://github.com/testcontainers/testcontainers-go) to:

1. Start a fresh ClickHouse container
2. Run all bundled migrations
3. Verify every table, view, and column was created correctly
4. Run migrations a second time to verify idempotency
5. Test rollback behavior with a deliberately failing migration

If using Podman instead of Docker:

```bash
DOCKER_HOST=unix:///run/user/$(id -u)/podman/podman.sock \
TESTCONTAINERS_RYUK_DISABLED=true \
make integration-test
```

---

## Deployment patterns

### One-shot job

Run the migrator as a Kubernetes Job before deploying your application:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: clickhouse-migrate
spec:
  template:
    spec:
      containers:
      - name: migrate
        image: your-registry/ch-migrator-example:latest
        args:
        - --dbAddr=$(CLICKHOUSE_HOST)
        - --dbPort=9440
        - --dbUsername=$(CLICKHOUSE_USER)
        - --dbPassword=$(DBPASSWORD)
        envFrom:
        - secretRef:
            name: clickhouse-credentials
      restartPolicy: Never
  backoffLimit: 3
```

### Scheduled CronJob

Run daily to enforce schema consistency:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: clickhouse-migrate-daily
spec:
  schedule: "0 2 * * *"
  concurrencyPolicy: Forbid
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: migrate
            image: your-registry/ch-migrator-example:latest
            args:
            - --dbAddr=$(CLICKHOUSE_HOST)
            - --dbPort=9440
          restartPolicy: Never
```

### Embedded in your application

Import the library and run migrations on startup:

```go
func main() {
    db := setupClickHouse()

    // Run migrations before starting the server
    cfg := migrations.Config{}
    if err := migrations.InitiateMigrations(db, cfg, migrations.EmbeddedMigrations); err != nil {
        log.Fatal("migration failed:", err)
    }

    startHTTPServer(db)
}
```

---

## Troubleshooting

### "Go functions must be registered and built into a custom binary"

The Go migrations weren't registered. If using the library, add a blank import:

```go
import _ "github.com/ClickHouse/ch-migrator-example/pkg/migrations"
```

### "duplicate filename detected in migrations"

Two migration files have the same name (even if in different subdirectories). The migrator flattens the directory structure — filenames must be globally unique.

### Migrations run but nothing changes

The migrator is idempotent. If migrations have already been applied, goose skips them. Check the current version:

```sql
SELECT * FROM goose_db_version ORDER BY version_id DESC LIMIT 5;
```

### "ERROR ... UNKNOWN_TABLE goose_db_version"

This is normal on first run — goose creates the tracking table automatically. The error appears in ClickHouse server logs but does not affect the migration.

### Rolling back a migration

Use `--revision` with a lower version number:

```bash
./bin/ch-migrator-example --revision 20260401000007 ...
```

This runs the `-- +goose Down` section of every migration between the current version and the target.

---

## Further reading

- [goose documentation](https://pressly.github.io/goose/) — the underlying migration framework
- [OTel Collector ClickHouse exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/clickhouseexporter) — the source of the bundled schemas
