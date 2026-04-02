-- +goose Up
CREATE DATABASE IF NOT EXISTS otel;

-- +goose Down
DROP DATABASE IF EXISTS otel;
