-- +goose Up
CREATE DATABASE IF NOT EXISTS test_migration_db;
CREATE TABLE test_migration_db.test_table1 (
                                               id UInt64,
                                               name String
) ENGINE = <SMT_ENGINE> ORDER BY id;

-- +goose Down
DROP TABLE IF EXISTS test_migration_db.test_table1;
DROP DATABASE IF EXISTS test_migration_db;
