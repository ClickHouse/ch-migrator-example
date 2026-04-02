-- +goose Up
CREATE MATERIALIZED VIEW IF NOT EXISTS otel.otel_traces_trace_id_ts_mv
TO otel.otel_traces_trace_id_ts
AS SELECT
    TraceId,
    min(Timestamp) as Start,
    max(Timestamp) as End
FROM otel.otel_traces
WHERE TraceId != ''
GROUP BY TraceId;

-- +goose Down
DROP VIEW IF EXISTS otel.otel_traces_trace_id_ts_mv;
