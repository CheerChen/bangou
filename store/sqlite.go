package store

import (
	"context"
	"database/sql"
	"errors"
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
	if _, err := db.Exec(schemaV1); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) UpsertFile(ctx context.Context, f *SourceFile) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO source_files (path, filename, size, ready, ignored)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(path) DO UPDATE SET
		   filename=excluded.filename,
		   size=excluded.size,
		   ready=excluded.ready`,
		f.Path, f.Filename, f.Size, f.Ready, f.Ignored,
	)
	return err
}

func (s *SQLiteStore) SetFileReady(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE source_files SET ready = TRUE WHERE path = ?`, path)
	return err
}

func (s *SQLiteStore) SetFileIgnored(ctx context.Context, fileID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE source_files SET ignored = TRUE WHERE id = ?`, fileID)
	return err
}

func (s *SQLiteStore) GetFileByPath(ctx context.Context, path string) (*SourceFile, error) {
	f := &SourceFile{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, path, filename, size, ready, ignored, created_at FROM source_files WHERE path = ?`, path,
	).Scan(&f.ID, &f.Path, &f.Filename, &f.Size, &f.Ready, &f.Ignored, &f.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *SQLiteStore) GetFileByID(ctx context.Context, id int64) (*SourceFile, error) {
	f := &SourceFile{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, path, filename, size, ready, ignored, created_at FROM source_files WHERE id = ?`, id,
	).Scan(&f.ID, &f.Path, &f.Filename, &f.Size, &f.Ready, &f.Ignored, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *SQLiteStore) ListUnreadyFiles(ctx context.Context) ([]SourceFile, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, path, filename, size, ready, ignored, created_at FROM source_files WHERE ready = FALSE AND ignored = FALSE`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []SourceFile
	for rows.Next() {
		var f SourceFile
		if err := rows.Scan(&f.ID, &f.Path, &f.Filename, &f.Size, &f.Ready, &f.Ignored, &f.CreatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return files, nil
}

func (s *SQLiteStore) ListUnknownFiles(ctx context.Context) ([]SourceFile, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT sf.id, sf.path, sf.filename, sf.size, sf.ready, sf.ignored, sf.created_at
		 FROM source_files sf
		 LEFT JOIN parsed_info pi ON pi.file_id = sf.id
		 WHERE sf.ready = TRUE AND sf.ignored = FALSE AND pi.id IS NULL
		 ORDER BY sf.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []SourceFile
	for rows.Next() {
		var f SourceFile
		if err := rows.Scan(&f.ID, &f.Path, &f.Filename, &f.Size, &f.Ready, &f.Ignored, &f.CreatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return files, nil
}

func (s *SQLiteStore) UpsertParsed(ctx context.Context, p *ParsedInfo) error {
	now := time.Now()
	if !p.UpdatedAt.IsZero() {
		now = p.UpdatedAt
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO parsed_info (file_id, number, part, source_site, tags, manual, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(file_id) DO UPDATE SET
		   number=excluded.number,
		   part=excluded.part,
		   source_site=excluded.source_site,
		   tags=excluded.tags,
		   manual=excluded.manual,
		   updated_at=excluded.updated_at`,
		p.FileID, p.Number, p.Part, p.SourceSite, p.Tags, p.Manual, now,
	)
	return err
}

func (s *SQLiteStore) GetParsedByFileID(ctx context.Context, fileID int64) (*ParsedInfo, error) {
	p := &ParsedInfo{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, file_id, number, part, source_site, tags, manual, updated_at FROM parsed_info WHERE file_id = ?`, fileID,
	).Scan(&p.ID, &p.FileID, &p.Number, &p.Part, &p.SourceSite, &p.Tags, &p.Manual, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *SQLiteStore) EnsureGroup(ctx context.Context, number string) (*Group, error) {
	now := time.Now()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO groups (number, created_at, updated_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(number) DO UPDATE SET updated_at = excluded.updated_at`,
		number, now, now,
	); err != nil {
		return nil, err
	}

	g := &Group{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, number, status, action, created_at, updated_at FROM groups WHERE number = ?`, number,
	).Scan(&g.ID, &g.Number, &g.Status, &g.Action, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (s *SQLiteStore) GetGroup(ctx context.Context, id int64) (*Group, error) {
	g := &Group{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, number, status, action, created_at, updated_at FROM groups WHERE id = ?`, id,
	).Scan(&g.ID, &g.Number, &g.Status, &g.Action, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (s *SQLiteStore) ListGroups(ctx context.Context, status string) ([]Group, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if status == "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, number, status, action, created_at, updated_at
			 FROM groups ORDER BY updated_at DESC`,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, number, status, action, created_at, updated_at
			 FROM groups WHERE status = ? ORDER BY updated_at DESC`,
			status,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Number, &g.Status, &g.Action, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *SQLiteStore) UpdateGroupAction(ctx context.Context, id int64, action string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE groups SET action = ?, updated_at = ? WHERE id = ?`,
		action, time.Now(), id,
	)
	return err
}

func (s *SQLiteStore) UpdateGroupStatus(ctx context.Context, id int64, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE groups SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	return err
}

func (s *SQLiteStore) GetGroupFiles(ctx context.Context, groupID int64) ([]SourceFile, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT sf.id, sf.path, sf.filename, sf.size, sf.ready, sf.ignored, sf.created_at
		 FROM source_files sf
		 JOIN parsed_info pi ON pi.file_id = sf.id
		 JOIN groups g ON g.number = pi.number
		 WHERE g.id = ?
		 ORDER BY pi.part ASC, sf.filename ASC`,
		groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []SourceFile
	for rows.Next() {
		var f SourceFile
		if err := rows.Scan(&f.ID, &f.Path, &f.Filename, &f.Size, &f.Ready, &f.Ignored, &f.CreatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return files, nil
}

func (s *SQLiteStore) CreateOutput(ctx context.Context, o *Output) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO outputs (group_id, link_path, link_type, alive, checked_at) VALUES (?, ?, ?, TRUE, ?)`,
		o.GroupID, o.LinkPath, o.LinkType, time.Now(),
	)
	return err
}

func (s *SQLiteStore) ListOutputs(ctx context.Context, groupID int64) ([]Output, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, group_id, link_path, link_type, alive, checked_at FROM outputs WHERE group_id = ? ORDER BY id DESC`,
		groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Output
	for rows.Next() {
		var o Output
		if err := rows.Scan(&o.ID, &o.GroupID, &o.LinkPath, &o.LinkType, &o.Alive, &o.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
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
		`SELECT id, group_id, link_path, link_type, alive, checked_at FROM outputs WHERE alive = FALSE`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Output
	for rows.Next() {
		var o Output
		if err := rows.Scan(&o.ID, &o.GroupID, &o.LinkPath, &o.LinkType, &o.Alive, &o.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *SQLiteStore) RecordMergedPart(ctx context.Context, m *MergedPart) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO merged_parts (group_id, filename, size, part) VALUES (?, ?, ?, ?)`,
		m.GroupID, m.Filename, m.Size, m.Part,
	)
	return err
}

func (s *SQLiteStore) GetMergedParts(ctx context.Context, groupID int64) ([]MergedPart, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, group_id, filename, size, part FROM merged_parts WHERE group_id = ? ORDER BY part ASC`,
		groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MergedPart
	for rows.Next() {
		var m MergedPart
		if err := rows.Scan(&m.ID, &m.GroupID, &m.Filename, &m.Size, &m.Part); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *SQLiteStore) GetMetadata(ctx context.Context, number string) (*Metadata, error) {
	m := &Metadata{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, number, title, plot, director, maker, label, series,
		        actors, genres, cover_url, cover_local, premiered, year, runtime,
				provider, scrape_status, scrape_errors, created_at, updated_at
		 FROM metadata WHERE number = ?`,
		number,
	).Scan(
		&m.ID, &m.Number, &m.Title, &m.Plot, &m.Director, &m.Maker, &m.Label, &m.Series,
		&m.Actors, &m.Genres, &m.CoverURL, &m.CoverLocal, &m.Premiered, &m.Year, &m.Runtime,
		&m.Provider, &m.ScrapeStatus, &m.ScrapeErrors, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (s *SQLiteStore) UpsertMetadata(ctx context.Context, m *Metadata) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO metadata (number, title, plot, director, maker, label, series,
		                       actors, genres, cover_url, cover_local, premiered, year, runtime,
							   provider, scrape_status, scrape_errors, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(number) DO UPDATE SET
		   title=excluded.title,
		   plot=excluded.plot,
		   director=excluded.director,
		   maker=excluded.maker,
		   label=excluded.label,
		   series=excluded.series,
		   actors=excluded.actors,
		   genres=excluded.genres,
		   cover_url=excluded.cover_url,
		   cover_local=excluded.cover_local,
		   premiered=excluded.premiered,
		   year=excluded.year,
		   runtime=excluded.runtime,
		   provider=excluded.provider,
		   scrape_status=excluded.scrape_status,
		   scrape_errors=excluded.scrape_errors,
		   updated_at=excluded.updated_at`,
		m.Number, m.Title, m.Plot, m.Director, m.Maker, m.Label, m.Series,
		m.Actors, m.Genres, m.CoverURL, m.CoverLocal, m.Premiered, m.Year, m.Runtime,
		m.Provider, m.ScrapeStatus, m.ScrapeErrors, time.Now(),
	)
	return err
}

func (s *SQLiteStore) SetMetadataStatus(ctx context.Context, number, status, scrapeErrors string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO metadata (number, scrape_status, scrape_errors, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(number) DO UPDATE SET
		   scrape_status=excluded.scrape_status,
		   scrape_errors=excluded.scrape_errors,
		   updated_at=excluded.updated_at`,
		number, status, scrapeErrors, time.Now(),
	)
	return err
}

func (s *SQLiteStore) GetSetting(ctx context.Context, key string) (string, error) {
	var val string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return val, err
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

	m := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		m[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return m, nil
}
