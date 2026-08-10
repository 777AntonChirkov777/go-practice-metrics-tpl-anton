-- +goose Up
CREATE TABLE IF NOT EXISTS gauges (
    id text PRIMARY KEY,
    value double precision NOT NULL
);

CREATE TABLE IF NOT EXISTS counters (
    id text PRIMARY KEY,
    delta bigint NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS counters;
DROP TABLE IF EXISTS gauges;
