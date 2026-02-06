CREATE DATABASE IF NOT EXISTS rca;

CREATE TABLE IF NOT EXISTS rca.logs (
    timestamp DateTime,
    service_name String,
    severity String,
    body String,
    trace_id String,
    span_id String,
    attributes Map(String, String)
) ENGINE = MergeTree()
ORDER BY (service_name, timestamp);

CREATE TABLE IF NOT EXISTS rca.traces (
    trace_id String,
    span_id String,
    parent_span_id String,
    service_name String,
    name String,
    start_time DateTime64(9),
    duration_ns UInt64,
    status_code String,
    status_message String,
    attributes Map(String, String),
    events String
) ENGINE = MergeTree()
ORDER BY (service_name, start_time);

CREATE TABLE IF NOT EXISTS rca.metrics (
    timestamp DateTime,
    name String,
    value Float64,
    labels Map(String, String)
) ENGINE = MergeTree()
ORDER BY (name, timestamp);
