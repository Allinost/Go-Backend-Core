-- Migration 1: Init
-- Created at 2024-01-01T00:00:00Z

CREATE TABLE IF NOT EXISTS _migrations (
    version INT NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    checksum VARCHAR(64) NOT NULL,
    PRIMARY KEY (version)
);
