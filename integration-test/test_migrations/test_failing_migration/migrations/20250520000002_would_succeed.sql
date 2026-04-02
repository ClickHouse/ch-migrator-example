-- +goose Up
CREATE TABLE test_migration_db.test_table3 (
                                               id UInt64,
                                               name String
) ENGINE = <SMT_ENGINE> ORDER BY id;

-- +goose Down
DROP TABLE IF EXISTS test_migration_db.test_table3;
