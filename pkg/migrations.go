package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"testing/fstest"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog/log"
)

const migrationsDir = "migrations"

func InitiateMigrations(db *sql.DB, mainCfg Config, fs fs.FS) error {
	_, err := db.Exec("SELECT 1")
	if err != nil {
		var exception *clickhouse.Exception
		if errors.As(err, &exception) {
			return fmt.Errorf("clickhouse exception: %s, stacktrace: %s", exception.Message, exception.StackTrace)
		}
		return fmt.Errorf("failure to SELECT 1 from DB | %w", err)
	}

	err = migrate(context.Background(), db, mainCfg.Revision, mainCfg.AllowMissing, fs)
	if err != nil {
		return fmt.Errorf("execute migrations: %w", err)
	}

	log.Info().Msg("migration completed successfully")
	return nil
}

func migrate(ctx context.Context, db *sql.DB, revision int64, allowMissing bool, rawFS fs.FS) error {
	// Flatten the potentially nested migrations directory into a single flat view
	// rooted at "."
	mapFS := make(fstest.MapFS)
	err := fs.WalkDir(rawFS, migrationsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "schemas" {
				return fs.SkipDir
			}
			return nil
		}
		// Flatten: use the filename only as the key
		flatName := d.Name()
		if _, exists := mapFS[flatName]; exists {
			return fmt.Errorf("duplicate filename detected in migrations: %s", flatName)
		}

		content, err := fs.ReadFile(rawFS, path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		mapFS[flatName] = &fstest.MapFile{Data: content}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to flatten migrations fs: %w", err)
	}

	goose.SetLogger(&zerologGooseLogger{})

	goose.SetBaseFS(mapFS)
	err = goose.SetDialect("clickhouse")
	if err != nil {
		log.Error().Err(err).Msg("goose does not know dialect clickhouse!")
		return err
	}

	startRevision, err := goose.EnsureDBVersion(db)
	if err != nil {
		log.Error().Err(err).Msg("unable to get current revision number")
		return err
	}
	log.Debug().Int("revision", int(startRevision)).Msg("migration from revision")

	// Goose should look at the root of our flattened FS
	const gooseDir = "."

	if revision == RevisionLatest {
		log.Info().Msg("migrating to latest")

		// if no revision was specified, migrate up latest
		if allowMissing {
			log.Debug().Msg("Running with allowMissing")
			err = goose.RunWithOptionsContext(ctx, "up", db, gooseDir, []string{}, goose.WithAllowMissing())
		} else {
			err = goose.RunContext(ctx, "up", db, gooseDir)
		}
	} else {
		// otherwise migrate up or down to specified revision
		direction := "up"
		if startRevision > revision {
			direction = "down"
		}

		log.Info().Str("direction", direction).Int64("revision", revision).Msg("migration to revision")
		directionCommand := fmt.Sprintf("%s-to", direction)
		revisionStr := strconv.FormatInt(revision, 10)
		if direction == "up" && allowMissing {
			log.Debug().Msg("Running with allowMissing")
			err = goose.RunWithOptionsContext(ctx, directionCommand, db, gooseDir, []string{revisionStr}, goose.WithAllowMissing())
		} else {
			err = goose.RunContext(ctx, directionCommand, db, gooseDir, revisionStr)
		}
	}

	if err != nil {
		log.Error().Err(err).Msg("unable to run migrations")
		return err
	}
	return nil
}

// zerologGooseLogger adapts zerolog to goose's Logger interface so that
// goose output appears as structured JSON instead of plain text.
type zerologGooseLogger struct{}

func (*zerologGooseLogger) Fatalf(format string, v ...interface{}) {
	log.Fatal().Msgf(format, v...)
}

func (*zerologGooseLogger) Printf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	msg = strings.TrimRight(msg, "\n")

	switch {
	case strings.HasPrefix(msg, "OK"):
		name, dur := parseGooseMigrationLine(msg)
		log.Info().Str("migration", name).Str("duration", dur).Msg("migration applied")
	case strings.HasPrefix(msg, "EMPTY"):
		name, dur := parseGooseMigrationLine(msg)
		log.Info().Str("migration", name).Str("duration", dur).Msg("migration empty")
	default:
		log.Info().Msg(msg)
	}
}

// parseGooseMigrationLine extracts the migration name and duration from
// goose log lines like "OK   20260401000001_create_otel_database.sql (5.85ms)"
func parseGooseMigrationLine(msg string) (name, duration string) {
	// Strip prefix ("OK   " or "EMPTY ")
	s := strings.TrimLeft(msg, "OKEPMTY ")
	if idx := strings.LastIndex(s, "("); idx > 0 {
		name = strings.TrimSpace(s[:idx])
		duration = strings.Trim(s[idx:], "() ")
	} else {
		name = strings.TrimSpace(s)
	}
	return
}
