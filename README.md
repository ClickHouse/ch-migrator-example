# ch-migrator-example

A reference implementation of a ClickHouse schema migration tool built on [goose](https://github.com/pressly/goose), with OpenTelemetry collector schemas as the example migration set. Fork it, replace the bundled migrations with your own, and you have a production-ready migrator for your ClickHouse deployment.

## What this does

This project provides a migration framework for ClickHouse databases that supports:

- **SQL migrations** with engine placeholder replacement (e.g., `<SMT_ENGINE>` becomes `SharedMergeTree()` in production or `MergeTree()` locally)
- **Go migrations** for dynamic, programmatic schema changes (e.g., altering TTL across tables matching a pattern)
- **Revision targeting** — migrate up to latest, or up/down to a specific revision
- **Rollback support** — each migration has an `up` and `down` path

The included migrations create the full [OpenTelemetry](https://opentelemetry.io/) schema for ClickHouse, as defined by the [OTel Collector ClickHouse exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/clickhouseexporter):

- **Logs** — `otel_logs` (Map attributes) and `otel_logs_json` (JSON attributes)
- **Traces** — `otel_traces`, `otel_traces_json`, trace ID timestamp lookup table + materialized view
- **Metrics** — gauge, sum, histogram, exponential histogram, summary

Plus two example Go migrations demonstrating:
- Setting TTL on metrics tables by pattern
- Adding a column to a table

## Quick start

Requires: Go 1.25+, Make, a running ClickHouse instance.

```bash
# Build
make build

# Run against a local ClickHouse (no TLS)
./bin/ch-migrator-example \
  --dbAddr localhost \
  --dbPort 9000 \
  --enableTLS=false \
  --forceMergeTree \
  --dbUsername default \
  --dbPassword ''
```

## Configuration

Configuration via CLI flags, environment variables, or YAML config file (`--config path/to/config.yaml`).

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--config` | `CONFIG` | | Path to YAML config file |
| `--dbAddr` | `DBADDR` | *(required)* | ClickHouse host |
| `--dbPort` | `DBPORT` | *(required)* | ClickHouse native port |
| `--dbUsername` | `DBUSERNAME` | `default` | User to run migrations |
| `--dbPassword` | `DBPASSWORD` | | Password |
| `--enableTLS` | `ENABLETLS` | `true` | Enable TLS |
| `--insecureSkipTLSVerify` | `INSECURESKIPTLSVERIFY` | `true` | Skip TLS verification |
| `--forceMergeTree` | `FORCEMERGETREE` | `false` | Use `MergeTree()` engine (required for local/single-node ClickHouse which does not support `SharedMergeTree` or `ReplicatedMergeTree`) |
| `--useHTTP` | `USEHTTP` | `false` | Use HTTP protocol instead of native (for HTTPS connections on port 8443) |
| `--allowMissing` | `ALLOWMISSING` | `false` | Allow out-of-order migrations (useful when branches add migrations that land in different order) |
| `--revision` | `REVISION` | `0` (latest) | Target migration revision |

## Engine placeholders

SQL migrations use placeholders that are replaced at runtime:

| Placeholder | Production | `--forceMergeTree` |
|---|---|---|
| `<SMT_ENGINE>` | `SharedMergeTree()` | `MergeTree()` |
| `<ReplacingMergeTree_ENGINE>` | `SharedReplacingMergeTree` | `ReplacingMergeTree` |
| `<SummingMergeTree_ENGINE>` | `SharedSummingMergeTree` | `SummingMergeTree` |
| `<AggregatingMergeTree_ENGINE>` | `SharedAggregatingMergeTree` | `AggregatingMergeTree` |
| `<CollapsingMergeTree_ENGINE>` | `SharedCollapsingMergeTree` | `CollapsingMergeTree` |

This is handled by the templated FS layer in `pkg/templateFS.go`, which wraps Go's `embed.FS` and applies string replacements on the fly.

## Using as a library

The migration package can be imported directly:

```go
import migrations "github.com/ClickHouse/ch-migrator-example/pkg"
```

Note: the directory is named `pkg` but the Go package is `migrations`, so an import alias is needed. The key exports are:

- `migrations.Config` — migration configuration struct
- `migrations.InitiateMigrations(db, config, fs)` — run migrations
- `migrations.EmbeddedMigrations` — the bundled OTel migration files
- `migrations.RevisionLatest` — sentinel for "migrate to latest"

## Adding your own migrations

See [`pkg/migrations/README.md`](pkg/migrations/README.md) for details on creating SQL and Go migrations.

## Testing

```bash
# Unit tests
make test

# Integration tests (requires Docker — uses testcontainers-go)
make integration-test
```
