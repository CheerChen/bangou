package committed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLite(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureCompatibleSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schemaV1); err != nil {
		db.Close()
		return nil, err
	}
	// v2 migration: add new metadata columns
	newCols := []struct{ name, def string }{
		{"sample_images", "TEXT NOT NULL DEFAULT ''"},
		{"rating", "TEXT NOT NULL DEFAULT ''"},
		{"review_count", "INTEGER NOT NULL DEFAULT 0"},
		{"page_url", "TEXT NOT NULL DEFAULT ''"},
		{"content_id", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, col := range newCols {
		if ok, _ := hasColumn(db, "metadata", col.name); !ok {
			if _, err := db.Exec(fmt.Sprintf("ALTER TABLE metadata ADD COLUMN %s %s", col.name, col.def)); err != nil {
				db.Close()
				return nil, fmt.Errorf("migrate metadata.%s: %w", col.name, err)
			}
		}
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) CreateOutput(ctx context.Context, o *Output) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO outputs (number, link_path, link_type, alive, created_at, checked_at)
		 VALUES (?, ?, ?, TRUE, ?, ?)`,
		o.Number, o.LinkPath, o.LinkType, time.Now(), time.Now(),
	)
	return err
}

func (s *SQLiteStore) DeleteOutput(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM outputs WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) ListOutputsByNumber(ctx context.Context, number string) ([]Output, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, number, link_path, link_type, alive, created_at, checked_at
		 FROM outputs WHERE number = ? ORDER BY id DESC`,
		number,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Output
	for rows.Next() {
		var o Output
		if err := rows.Scan(&o.ID, &o.Number, &o.LinkPath, &o.LinkType, &o.Alive, &o.CreatedAt, &o.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) ListAllOutputs(ctx context.Context) ([]Output, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, number, link_path, link_type, alive, created_at, checked_at
		 FROM outputs ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Output
	for rows.Next() {
		var o Output
		if err := rows.Scan(&o.ID, &o.Number, &o.LinkPath, &o.LinkType, &o.Alive, &o.CreatedAt, &o.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) SetOutputAlive(ctx context.Context, id int64, alive bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE outputs SET alive = ?, checked_at = ? WHERE id = ?`,
		alive, time.Now(), id,
	)
	return err
}

func (s *SQLiteStore) ListOrphanedOutputs(ctx context.Context) ([]Output, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, number, link_path, link_type, alive, created_at, checked_at
		 FROM outputs WHERE alive = FALSE ORDER BY checked_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Output
	for rows.Next() {
		var o Output
		if err := rows.Scan(&o.ID, &o.Number, &o.LinkPath, &o.LinkType, &o.Alive, &o.CreatedAt, &o.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) IsCommitted(ctx context.Context, number string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM outputs WHERE number = ? AND alive = TRUE`,
		number,
	).Scan(&count)
	return count > 0, err
}

func (s *SQLiteStore) RecordMergedParts(ctx context.Context, parts []MergedPart) error {
	if len(parts) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO merged_parts (number, filename, size, part, merged_at)
		 VALUES (?, ?, ?, ?, ?)`,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, p := range parts {
		if _, err := stmt.ExecContext(ctx, p.Number, p.Filename, p.Size, p.Part, now); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetMergedParts(ctx context.Context, number string) ([]MergedPart, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT number, filename, size, part FROM merged_parts WHERE number = ? ORDER BY part ASC, id ASC`,
		number,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MergedPart
	for rows.Next() {
		var p MergedPart
		if err := rows.Scan(&p.Number, &p.Filename, &p.Size, &p.Part); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) UpsertMetadata(ctx context.Context, m *Metadata) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO metadata (number, title, plot, director, maker, label, series,
		                       actors, genres, cover_url, sample_images, premiered, year, runtime,
		                       rating, review_count, page_url, content_id, provider, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(number) DO UPDATE SET
		   title=excluded.title, plot=excluded.plot, director=excluded.director,
		   maker=excluded.maker, label=excluded.label, series=excluded.series,
		   actors=excluded.actors, genres=excluded.genres, cover_url=excluded.cover_url,
		   sample_images=excluded.sample_images, premiered=excluded.premiered,
		   year=excluded.year, runtime=excluded.runtime, rating=excluded.rating,
		   review_count=excluded.review_count, page_url=excluded.page_url,
		   content_id=excluded.content_id, provider=excluded.provider,
		   updated_at=excluded.updated_at`,
		m.Number, m.Title, m.Plot, m.Director, m.Maker, m.Label, m.Series,
		m.Actors, m.Genres, m.CoverURL, m.SampleImages, m.Premiered, m.Year, m.Runtime,
		m.Rating, m.ReviewCount, m.PageURL, m.ContentID, m.Provider, time.Now(),
	)
	return err
}

func (s *SQLiteStore) GetMetadata(ctx context.Context, number string) (*Metadata, error) {
	m := &Metadata{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, number, title, plot, director, maker, label, series,
		        actors, genres, cover_url, sample_images, premiered, year, runtime,
		        rating, review_count, page_url, content_id, provider,
				created_at, updated_at
		 FROM metadata WHERE number = ?`,
		number,
	).Scan(&m.ID, &m.Number, &m.Title, &m.Plot, &m.Director, &m.Maker, &m.Label, &m.Series,
		&m.Actors, &m.Genres, &m.CoverURL, &m.SampleImages, &m.Premiered, &m.Year, &m.Runtime,
		&m.Rating, &m.ReviewCount, &m.PageURL, &m.ContentID, &m.Provider,
		&m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (s *SQLiteStore) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

func (s *SQLiteStore) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

func (s *SQLiteStore) GetAllSettings(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func ensureCompatibleSchema(db *sql.DB) error {
	if _, err := db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
		return err
	}
	defer func() { _, _ = db.Exec(`PRAGMA foreign_keys=ON`) }()

	if exists, err := tableExists(db, "outputs"); err != nil {
		return err
	} else if exists {
		ok, err := hasColumn(db, "outputs", "number")
		if err != nil {
			return err
		}
		if !ok {
			if _, err := db.Exec(`DROP TABLE IF EXISTS outputs`); err != nil {
				return err
			}
		}
	}

	if exists, err := tableExists(db, "merged_parts"); err != nil {
		return err
	} else if exists {
		ok, err := hasColumn(db, "merged_parts", "number")
		if err != nil {
			return err
		}
		if !ok {
			if _, err := db.Exec(`DROP TABLE IF EXISTS merged_parts`); err != nil {
				return err
			}
		}
	}

	// Remove obsolete pre-commit tables from old architecture.
	if _, err := db.Exec(`DROP TABLE IF EXISTS parsed_info`); err != nil {
		return err
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS source_files`); err != nil {
		return err
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS groups`); err != nil {
		return err
	}

	return nil
}

func tableExists(db *sql.DB, table string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count)
	return count > 0, err
}

func hasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid        int
			name       string
			ctype      string
			notnull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &defaultVal, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}
