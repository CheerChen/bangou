package committed

const schema = `
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

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

CREATE TABLE IF NOT EXISTS bangous (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    pipeline_id INTEGER NOT NULL REFERENCES pipelines(id),
    number      TEXT NOT NULL,
    out_dir     TEXT NOT NULL,
    nfo_path    TEXT NOT NULL DEFAULT '',
    cover_path  TEXT NOT NULL DEFAULT '',
    raw_path    TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(pipeline_id, number)
);

CREATE TABLE IF NOT EXISTS bangou_files (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    bangou_id   INTEGER NOT NULL REFERENCES bangous(id),
    src_path    TEXT NOT NULL DEFAULT '',
    link_path   TEXT NOT NULL,
    link_type   TEXT NOT NULL DEFAULT 'hardlink',
    file_size   INTEGER NOT NULL DEFAULT 0,
    resolution  TEXT NOT NULL DEFAULT '',
    video_codec TEXT NOT NULL DEFAULT '',
    audio_codec TEXT NOT NULL DEFAULT '',
    duration    TEXT NOT NULL DEFAULT '',
    bitrate     TEXT NOT NULL DEFAULT '',
    alive       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    checked_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_bangou_files_bangou ON bangou_files(bangou_id);

CREATE TABLE IF NOT EXISTS metadata (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    bangou_id    INTEGER NOT NULL UNIQUE REFERENCES bangous(id),
    number       TEXT NOT NULL DEFAULT '',
    title        TEXT NOT NULL DEFAULT '',
    plot         TEXT NOT NULL DEFAULT '',
    director     TEXT NOT NULL DEFAULT '',
    maker        TEXT NOT NULL DEFAULT '',
    label        TEXT NOT NULL DEFAULT '',
    series       TEXT NOT NULL DEFAULT '',
    actors       TEXT NOT NULL DEFAULT '',
    genres       TEXT NOT NULL DEFAULT '',
    cover_url    TEXT NOT NULL DEFAULT '',
    sample_images TEXT NOT NULL DEFAULT '',
    premiered    TEXT NOT NULL DEFAULT '',
    year         TEXT NOT NULL DEFAULT '',
    runtime      TEXT NOT NULL DEFAULT '',
    rating       TEXT NOT NULL DEFAULT '',
    review_count INTEGER NOT NULL DEFAULT 0,
    sample_movie_url TEXT NOT NULL DEFAULT '',
    page_url     TEXT NOT NULL DEFAULT '',
    content_id   TEXT NOT NULL DEFAULT '',
    provider     TEXT NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS merged_parts (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    number     TEXT NOT NULL,
    filename   TEXT NOT NULL,
    size       INTEGER NOT NULL DEFAULT 0,
    part       INTEGER NOT NULL DEFAULT 0,
    merged_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_merged_parts_number ON merged_parts(number);
`
