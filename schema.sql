-- ClickHouse Schema for Logs with Pattern Clustering

CREATE TABLE IF NOT EXISTS logs (
    timestamp DateTime64(9),
    application String,
    instance String,
    level String,
    message String,
    pattern_id UInt64,
    pattern_text String, -- Denormalized for easier querying, or join with a patterns table
    attributes Map(String, String),
    trace_id String
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (application, pattern_id, timestamp);