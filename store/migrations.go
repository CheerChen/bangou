package store

const schemaV1 = `
CREATE TABLE IF NOT EXISTS source_files (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    path       TEXT UNIQUE NOT NULL,
    filename   TEXT NOT NULL,
    size       INTEGER NOT NULL DEFAULT 0,
    ready      BOOLEAN NOT NULL DEFAULT FALSE,
    ignored    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS parsed_info (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id     INTEGER UNIQUE NOT NULL REFERENCES source_files(id),
    number      TEXT NOT NULL,
    part        INTEGER NOT NULL DEFAULT 0,
    source_site TEXT NOT NULL DEFAULT '',
    tags        TEXT NOT NULL DEFAULT '',
    manual      BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_parsed_number ON parsed_info(number);

CREATE TABLE IF NOT EXISTS groups (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    number     TEXT UNIQUE NOT NULL,
    status     TEXT NOT NULL DEFAULT 'pending',
    action     TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS outputs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id   INTEGER NOT NULL REFERENCES groups(id),
    link_path  TEXT NOT NULL,
    link_type  TEXT NOT NULL DEFAULT 'hardlink',
    alive      BOOLEAN NOT NULL DEFAULT TRUE,
    checked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS merged_parts (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id INTEGER NOT NULL REFERENCES groups(id),
    filename TEXT NOT NULL,
    size     INTEGER NOT NULL DEFAULT 0,
    part     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS metadata (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    number        TEXT UNIQUE NOT NULL,
    title         TEXT NOT NULL DEFAULT '',
    plot          TEXT NOT NULL DEFAULT '',
    director      TEXT NOT NULL DEFAULT '',
    maker         TEXT NOT NULL DEFAULT '',
    label         TEXT NOT NULL DEFAULT '',
    series        TEXT NOT NULL DEFAULT '',
    actors        TEXT NOT NULL DEFAULT '',
    genres        TEXT NOT NULL DEFAULT '',
    cover_url     TEXT NOT NULL DEFAULT '',
    cover_local   TEXT NOT NULL DEFAULT '',
    premiered     TEXT NOT NULL DEFAULT '',
    year          TEXT NOT NULL DEFAULT '',
    runtime       TEXT NOT NULL DEFAULT '',
    provider      TEXT NOT NULL DEFAULT '',
    scrape_status TEXT NOT NULL DEFAULT 'pending',
    scrape_errors TEXT NOT NULL DEFAULT '',
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`
