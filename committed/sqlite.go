package committed

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	// v1: base schema
	if _, err := db.Exec(schemaV1); err != nil {
		db.Close()
		return nil, err
	}
	// v2 column migration: src_path on outputs
	if ok, _ := hasColumn(db, "outputs", "src_path"); !ok {
		if _, err := db.Exec("ALTER TABLE outputs ADD COLUMN src_path TEXT NOT NULL DEFAULT ''"); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate outputs.src_path: %w", err)
		}
	}
	// v3 column migration: extra metadata columns
	metaCols := []struct{ name, def string }{
		{"sample_images", "TEXT NOT NULL DEFAULT ''"},
		{"rating", "TEXT NOT NULL DEFAULT ''"},
		{"review_count", "INTEGER NOT NULL DEFAULT 0"},
		{"page_url", "TEXT NOT NULL DEFAULT ''"},
		{"content_id", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, col := range metaCols {
		if ok, _ := hasColumn(db, "metadata", col.name); !ok {
			if _, err := db.Exec(fmt.Sprintf("ALTER TABLE metadata ADD COLUMN %s %s", col.name, col.def)); err != nil {
				db.Close()
				return nil, fmt.Errorf("migrate metadata.%s: %w", col.name, err)
			}
		}
	}
	// v4: pipelines + provider_configs tables, pipeline_id on outputs/metadata
	if _, err := db.Exec(schemaV2); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema v2: %w", err)
	}
	if ok, _ := hasColumn(db, "outputs", "pipeline_id"); !ok {
		if _, err := db.Exec("ALTER TABLE outputs ADD COLUMN pipeline_id INTEGER NOT NULL DEFAULT 0"); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate outputs.pipeline_id: %w", err)
		}
		if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_outputs_pipeline ON outputs(pipeline_id)"); err != nil {
			db.Close()
			return nil, err
		}
	}
	if ok, _ := hasColumn(db, "metadata", "pipeline_id"); !ok {
		if _, err := db.Exec("ALTER TABLE metadata ADD COLUMN pipeline_id INTEGER NOT NULL DEFAULT 0"); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate metadata.pipeline_id: %w", err)
		}
	}
	// v5 migration: media info columns on outputs
	mediaCols := []struct{ name, def string }{
		{"file_size", "INTEGER NOT NULL DEFAULT 0"},
		{"resolution", "TEXT NOT NULL DEFAULT ''"},
		{"video_codec", "TEXT NOT NULL DEFAULT ''"},
		{"audio_codec", "TEXT NOT NULL DEFAULT ''"},
		{"duration", "TEXT NOT NULL DEFAULT ''"},
		{"bitrate", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, col := range mediaCols {
		if ok, _ := hasColumn(db, "outputs", col.name); !ok {
			if _, err := db.Exec(fmt.Sprintf("ALTER TABLE outputs ADD COLUMN %s %s", col.name, col.def)); err != nil {
				db.Close()
				return nil, fmt.Errorf("migrate outputs.%s: %w", col.name, err)
			}
		}
	}

	// Migrate legacy settings into pipeline + provider_configs
	if err := migrateSettingsToPipeline(db); err != nil {
		log.Printf("warn: settings migration: %v", err)
	}

	return &SQLiteStore{db: db}, nil
}

// migrateSettingsToPipeline creates a default pipeline from legacy settings if none exist yet.
func migrateSettingsToPipeline(db *sql.DB) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM pipelines").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil // already migrated
	}
	// Read legacy settings
	settings := map[string]string{}
	rows, err := db.Query("SELECT key, value FROM settings")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		settings[k] = v
	}
	if err := rows.Err(); err != nil {
		return err
	}

	inputDir := settings["input_dir"]
	outputDir := settings["output_dir"]
	if inputDir == "" && outputDir == "" {
		return nil // fresh DB, nothing to migrate
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Build provider order
	providerOrder := strings.TrimSpace(settings["provider_order"])
	if providerOrder == "" {
		providerOrder = "avwiki,dmm"
	}

	pathPattern := strings.TrimSpace(settings["link_path_pattern"])
	if pathPattern == "" {
		pathPattern = "{Number}"
	}

	now := time.Now()
	res, err := tx.Exec(
		`INSERT INTO pipelines (name, input_dir, output_dir, path_pattern, scrape_providers, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"Default", inputDir, outputDir, pathPattern, providerOrder, now, now,
	)
	if err != nil {
		return fmt.Errorf("create default pipeline: %w", err)
	}
	pipelineID, _ := res.LastInsertId()

	// Update existing outputs/metadata
	if _, err := tx.Exec("UPDATE outputs SET pipeline_id = ? WHERE pipeline_id = 0", pipelineID); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE metadata SET pipeline_id = ? WHERE pipeline_id = 0", pipelineID); err != nil {
		return err
	}

	// Migrate DMM credentials
	dmmApiID := strings.TrimSpace(settings["dmm_api_id"])
	dmmAffID := strings.TrimSpace(settings["dmm_affiliate_id"])
	if dmmApiID != "" || dmmAffID != "" {
		cfg, _ := json.Marshal(map[string]string{"api_id": dmmApiID, "affiliate_id": dmmAffID})
		if _, err := tx.Exec("INSERT OR IGNORE INTO provider_configs (provider, config) VALUES ('dmm', ?)", string(cfg)); err != nil {
			return err
		}
	}

	// Migrate aria2 credentials
	aria2URL := strings.TrimSpace(settings["aria2_rpc_url"])
	aria2Token := strings.TrimSpace(settings["aria2_token"])
	if aria2URL != "" {
		cfg, _ := json.Marshal(map[string]string{"rpc_url": aria2URL, "token": aria2Token})
		if _, err := tx.Exec("INSERT OR IGNORE INTO provider_configs (provider, config) VALUES ('aria2', ?)", string(cfg)); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("migrated legacy settings to pipeline id=%d", pipelineID)
	return nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

// ── Pipelines ──

func (s *SQLiteStore) CreatePipeline(ctx context.Context, p *Pipeline) (int64, error) {
	now := time.Now()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO pipelines (name, input_dir, output_dir, path_pattern, archive_dir, enable_merge, download_provider, scrape_providers, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.InputDir, p.OutputDir, p.PathPattern, p.ArchiveDir, p.EnableMerge, p.DownloadProvider, p.ScrapeProviders, now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) DeletePipeline(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM pipelines WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) GetPipeline(ctx context.Context, id int64) (*Pipeline, error) {
	p := &Pipeline{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, input_dir, output_dir, path_pattern, archive_dir, enable_merge, download_provider, scrape_providers, created_at, updated_at
		 FROM pipelines WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.InputDir, &p.OutputDir, &p.PathPattern, &p.ArchiveDir, &p.EnableMerge, &p.DownloadProvider, &p.ScrapeProviders, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *SQLiteStore) ListPipelines(ctx context.Context) ([]Pipeline, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, input_dir, output_dir, path_pattern, archive_dir, enable_merge, download_provider, scrape_providers, created_at, updated_at
		 FROM pipelines ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Pipeline
	for rows.Next() {
		var p Pipeline
		if err := rows.Scan(&p.ID, &p.Name, &p.InputDir, &p.OutputDir, &p.PathPattern, &p.ArchiveDir, &p.EnableMerge, &p.DownloadProvider, &p.ScrapeProviders, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ── Provider Configs ──

func (s *SQLiteStore) GetProviderConfig(ctx context.Context, provider string) (string, error) {
	var config string
	err := s.db.QueryRowContext(ctx, `SELECT config FROM provider_configs WHERE provider = ?`, provider).Scan(&config)
	if errors.Is(err, sql.ErrNoRows) {
		return "{}", nil
	}
	return config, err
}

func (s *SQLiteStore) SetProviderConfig(ctx context.Context, provider, configJSON string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO provider_configs (provider, config) VALUES (?, ?)
		 ON CONFLICT(provider) DO UPDATE SET config = excluded.config`,
		provider, configJSON,
	)
	return err
}

func (s *SQLiteStore) ListProviderConfigs(ctx context.Context) ([]ProviderConfig, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT provider, config FROM provider_configs ORDER BY provider`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProviderConfig
	for rows.Next() {
		var pc ProviderConfig
		if err := rows.Scan(&pc.Provider, &pc.Config); err != nil {
			return nil, err
		}
		out = append(out, pc)
	}
	return out, rows.Err()
}

// ── Outputs ──

func (s *SQLiteStore) CreateOutput(ctx context.Context, o *Output) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO outputs (pipeline_id, number, src_path, link_path, link_type, file_size, resolution, video_codec, audio_codec, duration, bitrate, alive, created_at, checked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRUE, ?, ?)`,
		o.PipelineID, o.Number, o.SrcPath, o.LinkPath, o.LinkType,
		o.FileSize, o.Resolution, o.VideoCodec, o.AudioCodec, o.Duration, o.Bitrate,
		time.Now(), time.Now(),
	)
	return err
}

func (s *SQLiteStore) GetOutputByID(ctx context.Context, id int64) (*Output, error) {
	row, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s FROM outputs WHERE id = ?`, outputCols), id,
	)
	if err != nil {
		return nil, err
	}
	defer row.Close()
	if !row.Next() {
		return nil, sql.ErrNoRows
	}
	o, err := scanOutput(row)
	if err != nil {
		return nil, err
	}
	return &o, row.Err()
}

func (s *SQLiteStore) DeleteOutput(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM outputs WHERE id = ?`, id)
	return err
}

const outputCols = `id, pipeline_id, number, src_path, link_path, link_type, file_size, resolution, video_codec, audio_codec, duration, bitrate, alive, created_at, checked_at`

func scanOutput(rows *sql.Rows) (Output, error) {
	var o Output
	err := rows.Scan(&o.ID, &o.PipelineID, &o.Number, &o.SrcPath, &o.LinkPath, &o.LinkType,
		&o.FileSize, &o.Resolution, &o.VideoCodec, &o.AudioCodec, &o.Duration, &o.Bitrate,
		&o.Alive, &o.CreatedAt, &o.CheckedAt)
	return o, err
}

func resolveOutputOrder(sort, order string) string {
	dir := "DESC"
	if order == "asc" {
		dir = "ASC"
	}
	switch sort {
	case "number":
		return "o.number " + dir
	case "date":
		return "COALESCE(m.premiered, m.year, '') " + dir + ", o.created_at DESC"
	case "rating":
		return "CAST(COALESCE(NULLIF(m.rating,''),'0') AS REAL) " + dir + ", o.created_at DESC"
	default: // "added"
		return "o.created_at " + dir
	}
}

func resolveOutputGroupOrder(sort, order string) string {
	dir := "DESC"
	if order == "asc" {
		dir = "ASC"
	}
	switch sort {
	case "number":
		return "o.number " + dir
	case "date":
		return "COALESCE(MAX(m.premiered), MAX(m.year), '') " + dir + ", MAX(o.created_at) DESC"
	case "rating":
		return "CAST(COALESCE(NULLIF(MAX(m.rating),''),'0') AS REAL) " + dir + ", MAX(o.created_at) DESC"
	default: // "added"
		return "MAX(o.created_at) " + dir
	}
}

func (s *SQLiteStore) ListOutputsByPipeline(ctx context.Context, pipelineID int64, limit, offset int, sort, order string) ([]Output, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outputs WHERE pipeline_id = ?`, pipelineID).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		return nil, total, nil
	}

	orderClause := resolveOutputOrder(sort, order)

	query := fmt.Sprintf(
		`SELECT o.id, o.pipeline_id, o.number, o.src_path, o.link_path, o.link_type,
		        o.file_size, o.resolution, o.video_codec, o.audio_codec, o.duration, o.bitrate,
		        o.alive, o.created_at, o.checked_at
		 FROM outputs o LEFT JOIN metadata m ON o.number = m.number
		 WHERE o.pipeline_id = ?
		 ORDER BY %s
		 LIMIT ? OFFSET ?`, orderClause)

	rows, err := s.db.QueryContext(ctx, query, pipelineID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Output
	for rows.Next() {
		var o Output
		if err := rows.Scan(&o.ID, &o.PipelineID, &o.Number, &o.SrcPath, &o.LinkPath, &o.LinkType,
			&o.FileSize, &o.Resolution, &o.VideoCodec, &o.AudioCodec, &o.Duration, &o.Bitrate,
			&o.Alive, &o.CreatedAt, &o.CheckedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, o)
	}
	return out, total, rows.Err()
}

func (s *SQLiteStore) ListOutputGroupsByPipeline(ctx context.Context, pipelineID int64, limit, offset int, sort, order string) ([]OutputGroup, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT number) FROM outputs WHERE pipeline_id = ?`, pipelineID).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		return nil, total, nil
	}

	orderClause := resolveOutputGroupOrder(sort, order)
	groupNumberQuery := fmt.Sprintf(
		`SELECT o.number
		 FROM outputs o
		 LEFT JOIN metadata m ON o.number = m.number
		 WHERE o.pipeline_id = ?
		 GROUP BY o.number
		 ORDER BY %s
		 LIMIT ? OFFSET ?`, orderClause)

	numberRows, err := s.db.QueryContext(ctx, groupNumberQuery, pipelineID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer numberRows.Close()

	numbers := make([]string, 0, limit)
	for numberRows.Next() {
		var number string
		if err := numberRows.Scan(&number); err != nil {
			return nil, 0, err
		}
		numbers = append(numbers, number)
	}
	if err := numberRows.Err(); err != nil {
		return nil, 0, err
	}
	if len(numbers) == 0 {
		return []OutputGroup{}, total, nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(numbers)), ",")
	args := make([]any, 0, len(numbers)+1)
	args = append(args, pipelineID)
	for _, n := range numbers {
		args = append(args, n)
	}

	outputQuery := fmt.Sprintf(
		`SELECT %s
		 FROM outputs
		 WHERE pipeline_id = ? AND number IN (%s)
		 ORDER BY created_at DESC, id DESC`, outputCols, placeholders)
	rows, err := s.db.QueryContext(ctx, outputQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	byNumber := make(map[string][]Output, len(numbers))
	for rows.Next() {
		o, err := scanOutput(rows)
		if err != nil {
			return nil, 0, err
		}
		byNumber[o.Number] = append(byNumber[o.Number], o)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	groups := make([]OutputGroup, 0, len(numbers))
	for _, number := range numbers {
		groups = append(groups, OutputGroup{
			Number:  number,
			Outputs: byNumber[number],
		})
	}
	return groups, total, nil
}

func (s *SQLiteStore) ListAllOutputs(ctx context.Context) ([]Output, error) {
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s FROM outputs ORDER BY created_at DESC`, outputCols),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Output
	for rows.Next() {
		o, err := scanOutput(rows)
		if err != nil {
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

func (s *SQLiteStore) SetOutputLinkType(ctx context.Context, id int64, linkType string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE outputs SET link_type = ? WHERE id = ?`, linkType, id)
	return err
}

func (s *SQLiteStore) SetOutputSrcPath(ctx context.Context, id int64, srcPath string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE outputs SET src_path = ? WHERE id = ?`, srcPath, id)
	return err
}

func (s *SQLiteStore) SetOutputMedia(ctx context.Context, id int64, fileSize int64, resolution, videoCodec, audioCodec, duration, bitrate string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE outputs SET file_size=?, resolution=?, video_codec=?, audio_codec=?, duration=?, bitrate=? WHERE id=?`,
		fileSize, resolution, videoCodec, audioCodec, duration, bitrate, id)
	return err
}

func (s *SQLiteStore) ListOrphanedOutputs(ctx context.Context) ([]Output, error) {
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s FROM outputs WHERE alive = FALSE ORDER BY checked_at DESC`, outputCols),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Output
	for rows.Next() {
		o, err := scanOutput(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) IsCommitted(ctx context.Context, number string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM outputs WHERE number = ? AND alive = TRUE`, number,
	).Scan(&count)
	return count > 0, err
}

// ── Merged Parts ──

func (s *SQLiteStore) RecordMergedParts(ctx context.Context, parts []MergedPart) error {
	if len(parts) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO merged_parts (number, filename, size, part, merged_at) VALUES (?, ?, ?, ?, ?)`,
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
		`SELECT number, filename, size, part FROM merged_parts WHERE number = ? ORDER BY part ASC, id ASC`, number,
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

// ── Metadata ──

func (s *SQLiteStore) UpsertMetadata(ctx context.Context, m *Metadata) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO metadata (pipeline_id, number, title, plot, director, maker, label, series,
		                       actors, genres, cover_url, sample_images, premiered, year, runtime,
		                       rating, review_count, page_url, content_id, provider, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(number) DO UPDATE SET
		   pipeline_id=excluded.pipeline_id,
		   title=excluded.title, plot=excluded.plot, director=excluded.director,
		   maker=excluded.maker, label=excluded.label, series=excluded.series,
		   actors=excluded.actors, genres=excluded.genres, cover_url=excluded.cover_url,
		   sample_images=excluded.sample_images, premiered=excluded.premiered,
		   year=excluded.year, runtime=excluded.runtime, rating=excluded.rating,
		   review_count=excluded.review_count, page_url=excluded.page_url,
		   content_id=excluded.content_id, provider=excluded.provider,
		   updated_at=excluded.updated_at`,
		m.PipelineID, m.Number, m.Title, m.Plot, m.Director, m.Maker, m.Label, m.Series,
		m.Actors, m.Genres, m.CoverURL, m.SampleImages, m.Premiered, m.Year, m.Runtime,
		m.Rating, m.ReviewCount, m.PageURL, m.ContentID, m.Provider, time.Now(),
	)
	return err
}

func (s *SQLiteStore) GetMetadata(ctx context.Context, number string) (*Metadata, error) {
	m := &Metadata{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, pipeline_id, number, title, plot, director, maker, label, series,
		        actors, genres, cover_url, sample_images, premiered, year, runtime,
		        rating, review_count, page_url, content_id, provider,
		        created_at, updated_at
		 FROM metadata WHERE number = ?`, number,
	).Scan(&m.ID, &m.PipelineID, &m.Number, &m.Title, &m.Plot, &m.Director, &m.Maker, &m.Label, &m.Series,
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

// ── Legacy Settings ──

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
		`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
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

// ── Helpers ──

func ensureCompatibleSchema(db *sql.DB) error {
	if _, err := db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
		return err
	}
	defer func() { _, _ = db.Exec(`PRAGMA foreign_keys=ON`) }()

	if exists, err := tableExists(db, "outputs"); err != nil {
		return err
	} else if exists {
		if ok, err := hasColumn(db, "outputs", "number"); err != nil {
			return err
		} else if !ok {
			if _, err := db.Exec(`DROP TABLE IF EXISTS outputs`); err != nil {
				return err
			}
		}
	}

	if exists, err := tableExists(db, "merged_parts"); err != nil {
		return err
	} else if exists {
		if ok, err := hasColumn(db, "merged_parts", "number"); err != nil {
			return err
		} else if !ok {
			if _, err := db.Exec(`DROP TABLE IF EXISTS merged_parts`); err != nil {
				return err
			}
		}
	}

	for _, t := range []string{"parsed_info", "source_files", "groups"} {
		if _, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", t)); err != nil {
			return err
		}
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
	return false, rows.Err()
}
