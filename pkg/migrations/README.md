# Migrations

This directory (`pkg/migrations/`) contains database migrations managed by [goose](https://github.com/pressly/goose).

All migration files — both `.sql` and `.go` — must be placed directly in this directory.

## Adding a new migration

Requires the [goose CLI](https://pressly.github.io/goose/installation/).

### SQL migration

```bash
make new-migration NAME=add_user_events
```

The file should use goose annotations:

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS otel.my_table (...) ENGINE = MergeTree();

-- +goose Down
DROP TABLE IF EXISTS otel.my_table;
```

SQL files are embedded at compile time via `//go:embed migrations` in `pkg/config.go`.

### Go migration

```bash
make new-go-migration NAME=set_ttl
```

This creates a `.go` file in this directory with `package migrations` and an `init()` function that registers the migration with goose. The `init()` function runs automatically when the package is imported:

```go
package migrations

import (
    "context"
    "database/sql"
    "github.com/pressly/goose/v3"
)

func init() {
    goose.AddMigrationContext(up, down)
}

func up(ctx context.Context, tx *sql.Tx) error {
    // your migration logic
    return nil
}

func down(ctx context.Context, tx *sql.Tx) error {
    // your rollback logic
    return nil
}
```

Go migrations are registered via a blank import (`_ "github.com/ClickHouse/ch-migrator-example/pkg/migrations"`) in the binary's `main.go`. Library consumers who want to use their own Go migrations can omit this import and register their own.
