package committed

const schemaV1 = `
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS outputs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    number     TEXT NOT NULL,
    link_path  TEXT NOT NULL,
    link_type  TEXT NOT NULL DEFAULT 'hardlink',
    alive      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    checked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_outputs_number ON outputs(number);

CREATE TABLE IF NOT EXISTS merged_parts (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    number     TEXT NOT NULL,
    filename   TEXT NOT NULL,
    size       INTEGER NOT NULL DEFAULT 0,
    part       INTEGER NOT NULL DEFAULT 0,
    merged_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_merged_parts_number ON merged_parts(number);

CREATE TABLE IF NOT EXISTS metadata (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    number     TEXT UNIQUE NOT NULL,
    title      TEXT NOT NULL DEFAULT '',
    plot       TEXT NOT NULL DEFAULT '',
    director   TEXT NOT NULL DEFAULT '',
    maker      TEXT NOT NULL DEFAULT '',
    label      TEXT NOT NULL DEFAULT '',
    series     TEXT NOT NULL DEFAULT '',
    actors     TEXT NOT NULL DEFAULT '',
    genres     TEXT NOT NULL DEFAULT '',
    cover_url  TEXT NOT NULL DEFAULT '',
    premiered  TEXT NOT NULL DEFAULT '',
    year       TEXT NOT NULL DEFAULT '',
    runtime    TEXT NOT NULL DEFAULT '',
    provider   TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

const schemaV2 = `
CREATE TABLE IF NOT EXISTS pipelines (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    name              TEXT NOT NULL,
    input_dir         TEXT NOT NULL UNIQUE,
    output_dir        TEXT NOT NULL,
    path_pattern      TEXT NOT NULL DEFAULT '{Number}',
    archive_dir       TEXT NOT NULL DEFAULT '',
    enable_merge      BOOLEAN NOT NULL DEFAULT FALSE,
    download_provider TEXT NOT NULL DEFAULT 'none',
    scrape_providers  TEXT NOT NULL DEFAULT 'avwiki,dmm',
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS provider_configs (
    provider TEXT PRIMARY KEY,
    config   TEXT NOT NULL DEFAULT '{}'
);
`
