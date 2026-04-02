package test_failing_migration

import "embed"

//go:embed migrations/*.sql
var TestFailingMigrations embed.FS
