-- +goose Up
CREATE TABLE test_migration_db.test_table2 (
                                               id UInt64,
                                               name String
) ENGINE = THIS_ENGINE_DOES_NOT_EXIST; -- intentional error to cause migration to fail

-- +goose Down
DROP TABLE IF EXISTS test_migration_db.test_table2;
