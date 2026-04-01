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

	// v6: bangou refactor — create new tables, migrate data, then ensure new metadata schema
	if _, err := db.Exec(schemaV3Tables); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema v3 tables: %w", err)
	}
	if err := migrateOutputsToBangou(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate to bangou: %w", err)
	}
	// After migration drops old metadata, ensure new metadata table exists
	if _, err := db.Exec(schemaV3Metadata); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema v3 metadata: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// migrateOutputsToBangou migrates data from the legacy outputs+metadata tables
// into the new bangous+bangou_files+metadata_v2 tables. One-time migration.
func migrateOutputsToBangou(db *sql.DB) error {
	// Skip if outputs table no longer exists (already migrated or fresh DB)
	exists, err := tableExists(db, "outputs")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	// Skip if outputs table is empty AND bangous already has data
	var outputCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM outputs").Scan(&outputCount); err != nil {
		return err
	}
	if outputCount == 0 {
		// No data to migrate; drop legacy tables
		for _, t := range []string{"outputs", "metadata"} {
			if _, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", t)); err != nil {
				return err
			}
		}
		return nil
	}

	return migrateOutputsToBangouGo(db)
}

// migrateOutputsToBangouGo is the Go-driven fallback for the migration,
// used when pure-SQL dirname extraction fails.
func migrateOutputsToBangouGo(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Read all distinct (pipeline_id, number) with a representative link_path
	rows, err := tx.Query(`
		SELECT pipeline_id, number, MIN(link_path) AS link_path
		FROM outputs GROUP BY pipeline_id, number`)
	if err != nil {
		return err
	}
	type groupRow struct {
		pipelineID       int64
		number, linkPath string
	}
	var groups []groupRow
	for rows.Next() {
		var g groupRow
		if err := rows.Scan(&g.pipelineID, &g.number, &g.linkPath); err != nil {
			rows.Close()
			return err
		}
		groups = append(groups, g)
	}
	rows.Close()

	// 2. Insert bangous with Go-computed dirname
	now := time.Now()
	bangouIDs := map[string]int64{} // "pipelineID:number" → bangou ID
	for _, g := range groups {
		outDir := dirName(g.linkPath)
		res, err := tx.Exec(
			`INSERT INTO bangous (pipeline_id, number, out_dir, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
			g.pipelineID, g.number, outDir, now, now)
		if err != nil {
			return fmt.Errorf("insert bangou %s: %w", g.number, err)
		}
		id, _ := res.LastInsertId()
		bangouIDs[fmt.Sprintf("%d:%s", g.pipelineID, g.number)] = id
	}

	// 3. Migrate outputs → bangou_files
	outputRows, err := tx.Query(`SELECT pipeline_id, number, src_path, link_path, link_type,
	                                    file_size, resolution, video_codec, audio_codec, duration, bitrate,
	                                    alive, created_at, checked_at FROM outputs`)
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO bangou_files (bangou_id, src_path, link_path, link_type,
	                           file_size, resolution, video_codec, audio_codec, duration, bitrate,
	                           alive, created_at, checked_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		outputRows.Close()
		return err
	}
	defer stmt.Close()
	for outputRows.Next() {
		var pipelineID int64
		var number, srcPath, linkPath, linkType string
		var fileSize int64
		var resolution, videoCodec, audioCodec, duration, bitrate string
		var alive bool
		var createdAt, checkedAt time.Time
		if err := outputRows.Scan(&pipelineID, &number, &srcPath, &linkPath, &linkType,
			&fileSize, &resolution, &videoCodec, &audioCodec, &duration, &bitrate,
			&alive, &createdAt, &checkedAt); err != nil {
			outputRows.Close()
			return err
		}
		bangouID := bangouIDs[fmt.Sprintf("%d:%s", pipelineID, number)]
		if _, err := stmt.Exec(bangouID, srcPath, linkPath, linkType,
			fileSize, resolution, videoCodec, audioCodec, duration, bitrate,
			alive, createdAt, checkedAt); err != nil {
			outputRows.Close()
			return err
		}
	}
	outputRows.Close()

	// 4. Rebuild metadata with bangou_id
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS metadata_v2 (
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
		    page_url     TEXT NOT NULL DEFAULT '',
		    content_id   TEXT NOT NULL DEFAULT '',
		    provider     TEXT NOT NULL DEFAULT '',
		    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("create metadata_v2: %w", err)
	}

	metaRows, err := tx.Query(`SELECT pipeline_id, number, title, plot, director, maker, label, series,
	                                  actors, genres, cover_url, sample_images, premiered, year, runtime,
	                                  rating, review_count, page_url, content_id, provider,
	                                  created_at, updated_at FROM metadata`)
	if err != nil {
		return fmt.Errorf("read metadata: %w", err)
	}
	metaStmt, err := tx.Prepare(`INSERT OR IGNORE INTO metadata_v2
		(bangou_id, number, title, plot, director, maker, label, series,
		 actors, genres, cover_url, sample_images, premiered, year, runtime,
		 rating, review_count, page_url, content_id, provider, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		metaRows.Close()
		return err
	}
	defer metaStmt.Close()
	for metaRows.Next() {
		var m Metadata
		var pipelineID int64
		if err := metaRows.Scan(&pipelineID, &m.Number, &m.Title, &m.Plot, &m.Director, &m.Maker, &m.Label, &m.Series,
			&m.Actors, &m.Genres, &m.CoverURL, &m.SampleImages, &m.Premiered, &m.Year, &m.Runtime,
			&m.Rating, &m.ReviewCount, &m.PageURL, &m.ContentID, &m.Provider,
			&m.CreatedAt, &m.UpdatedAt); err != nil {
			metaRows.Close()
			return err
		}
		bangouID, ok := bangouIDs[fmt.Sprintf("%d:%s", pipelineID, m.Number)]
		if !ok {
			continue // metadata without matching output, skip
		}
		if _, err := metaStmt.Exec(bangouID, m.Number, m.Title, m.Plot, m.Director, m.Maker, m.Label, m.Series,
			m.Actors, m.Genres, m.CoverURL, m.SampleImages, m.Premiered, m.Year, m.Runtime,
			m.Rating, m.ReviewCount, m.PageURL, m.ContentID, m.Provider, m.CreatedAt, m.UpdatedAt); err != nil {
			metaRows.Close()
			return err
		}
	}
	metaRows.Close()

	// 5. Drop old tables, rename new
	for _, s := range []string{
		"DROP TABLE IF EXISTS outputs",
		"DROP TABLE IF EXISTS metadata",
		"ALTER TABLE metadata_v2 RENAME TO metadata",
	} {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("finalize: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("migrated outputs+metadata to bangous+bangou_files+metadata (Go path)")
	return nil
}

// dirName returns the directory portion of a path (pure string, no OS calls).
func dirName(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		i = strings.LastIndex(path, "\\")
	}
	if i < 0 {
		return "."
	}
	return path[:i]
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

	// Update existing outputs/metadata (tables may not exist after bangou migration)
	if exists, _ := tableExists(tx, "outputs"); exists {
		if _, err := tx.Exec("UPDATE outputs SET pipeline_id = ? WHERE pipeline_id = 0", pipelineID); err != nil {
			return err
		}
	}
	if exists, _ := tableExists(tx, "metadata"); exists {
		if ok, _ := hasColumn(tx, "metadata", "pipeline_id"); ok {
			if _, err := tx.Exec("UPDATE metadata SET pipeline_id = ? WHERE pipeline_id = 0", pipelineID); err != nil {
				return err
			}
		}
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

// ── Bangous ──

func (s *SQLiteStore) CreateBangou(ctx context.Context, b *Bangou) (int64, error) {
	now := time.Now()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO bangous (pipeline_id, number, out_dir, nfo_path, cover_path, raw_path, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		b.PipelineID, b.Number, b.OutDir, b.NFOPath, b.CoverPath, b.RawPath, now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) GetBangou(ctx context.Context, id int64) (*Bangou, error) {
	b := &Bangou{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, pipeline_id, number, out_dir, nfo_path, cover_path, raw_path, created_at, updated_at
		 FROM bangous WHERE id = ?`, id,
	).Scan(&b.ID, &b.PipelineID, &b.Number, &b.OutDir, &b.NFOPath, &b.CoverPath, &b.RawPath, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (s *SQLiteStore) GetBangouByPipelineAndNumber(ctx context.Context, pipelineID int64, number string) (*Bangou, error) {
	b := &Bangou{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, pipeline_id, number, out_dir, nfo_path, cover_path, raw_path, created_at, updated_at
		 FROM bangous WHERE pipeline_id = ? AND number = ?`, pipelineID, number,
	).Scan(&b.ID, &b.PipelineID, &b.Number, &b.OutDir, &b.NFOPath, &b.CoverPath, &b.RawPath, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (s *SQLiteStore) UpdateBangouPaths(ctx context.Context, id int64, nfoPath, coverPath, rawPath string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE bangous SET nfo_path = ?, cover_path = ?, raw_path = ?, updated_at = ? WHERE id = ?`,
		nfoPath, coverPath, rawPath, time.Now(), id,
	)
	return err
}

func (s *SQLiteStore) DeleteBangou(ctx context.Context, id int64) error {
	// Delete associated metadata and files first (cascade)
	if _, err := s.db.ExecContext(ctx, `DELETE FROM metadata WHERE bangou_id = ?`, id); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM bangou_files WHERE bangou_id = ?`, id); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM bangous WHERE id = ?`, id)
	return err
}

func resolveBangouOrder(sort, order string) string {
	dir := "DESC"
	if order == "asc" {
		dir = "ASC"
	}
	switch sort {
	case "number":
		return "b.number " + dir
	case "date":
		return "COALESCE(m.premiered, m.year, '') " + dir + ", b.created_at DESC"
	case "rating":
		return "CAST(COALESCE(NULLIF(m.rating,''),'0') AS REAL) " + dir + ", b.created_at DESC"
	default: // "added"
		return "b.created_at " + dir
	}
}

func (s *SQLiteStore) ListBangousByPipeline(ctx context.Context, pipelineID int64, limit, offset int, sort, order string) ([]Bangou, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bangous WHERE pipeline_id = ?`, pipelineID).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		return nil, total, nil
	}

	orderClause := resolveBangouOrder(sort, order)
	query := fmt.Sprintf(
		`SELECT b.id, b.pipeline_id, b.number, b.out_dir, b.nfo_path, b.cover_path, b.raw_path, b.created_at, b.updated_at
		 FROM bangous b
		 LEFT JOIN metadata m ON b.id = m.bangou_id
		 WHERE b.pipeline_id = ?
		 ORDER BY %s
		 LIMIT ? OFFSET ?`, orderClause)

	rows, err := s.db.QueryContext(ctx, query, pipelineID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Bangou
	for rows.Next() {
		var b Bangou
		if err := rows.Scan(&b.ID, &b.PipelineID, &b.Number, &b.OutDir, &b.NFOPath, &b.CoverPath, &b.RawPath, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, b)
	}
	return out, total, rows.Err()
}

func (s *SQLiteStore) IsBangouCommitted(ctx context.Context, pipelineID int64, number string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM bangous b
		 JOIN bangou_files bf ON bf.bangou_id = b.id
		 WHERE b.pipeline_id = ? AND b.number = ? AND bf.alive = TRUE`,
		pipelineID, number,
	).Scan(&count)
	return count > 0, err
}

// ── Bangou Files ──

func (s *SQLiteStore) CreateBangouFile(ctx context.Context, f *BangouFile) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO bangou_files (bangou_id, src_path, link_path, link_type, file_size,
		                           resolution, video_codec, audio_codec, duration, bitrate,
		                           alive, created_at, checked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRUE, ?, ?)`,
		f.BangouID, f.SrcPath, f.LinkPath, f.LinkType, f.FileSize,
		f.Resolution, f.VideoCodec, f.AudioCodec, f.Duration, f.Bitrate,
		time.Now(), time.Now(),
	)
	return err
}

const bangouFileCols = `id, bangou_id, src_path, link_path, link_type, file_size, resolution, video_codec, audio_codec, duration, bitrate, alive, created_at, checked_at`

func scanBangouFile(rows *sql.Rows) (BangouFile, error) {
	var f BangouFile
	err := rows.Scan(&f.ID, &f.BangouID, &f.SrcPath, &f.LinkPath, &f.LinkType,
		&f.FileSize, &f.Resolution, &f.VideoCodec, &f.AudioCodec, &f.Duration, &f.Bitrate,
		&f.Alive, &f.CreatedAt, &f.CheckedAt)
	return f, err
}

func (s *SQLiteStore) GetBangouFileByID(ctx context.Context, id int64) (*BangouFile, error) {
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s FROM bangou_files WHERE id = ?`, bangouFileCols), id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	f, err := scanBangouFile(rows)
	if err != nil {
		return nil, err
	}
	return &f, rows.Err()
}

func (s *SQLiteStore) DeleteBangouFile(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM bangou_files WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) ListBangouFilesByBangou(ctx context.Context, bangouID int64) ([]BangouFile, error) {
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s FROM bangou_files WHERE bangou_id = ? ORDER BY created_at ASC`, bangouFileCols), bangouID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BangouFile
	for rows.Next() {
		f, err := scanBangouFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) ListAllBangouFiles(ctx context.Context) ([]BangouFile, error) {
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s FROM bangou_files ORDER BY created_at DESC`, bangouFileCols),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BangouFile
	for rows.Next() {
		f, err := scanBangouFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) SetBangouFileAlive(ctx context.Context, id int64, alive bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE bangou_files SET alive = ?, checked_at = ? WHERE id = ?`,
		alive, time.Now(), id,
	)
	return err
}

func (s *SQLiteStore) SetBangouFileLinkType(ctx context.Context, id int64, linkType string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE bangou_files SET link_type = ? WHERE id = ?`, linkType, id)
	return err
}

func (s *SQLiteStore) SetBangouFileSrcPath(ctx context.Context, id int64, srcPath string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE bangou_files SET src_path = ? WHERE id = ?`, srcPath, id)
	return err
}

func (s *SQLiteStore) SetBangouFileMedia(ctx context.Context, id int64, fileSize int64, resolution, videoCodec, audioCodec, duration, bitrate string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE bangou_files SET file_size=?, resolution=?, video_codec=?, audio_codec=?, duration=?, bitrate=? WHERE id=?`,
		fileSize, resolution, videoCodec, audioCodec, duration, bitrate, id)
	return err
}

func (s *SQLiteStore) ListOrphanedBangouFiles(ctx context.Context) ([]BangouFile, error) {
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT %s FROM bangou_files WHERE alive = FALSE ORDER BY checked_at DESC`, bangouFileCols),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BangouFile
	for rows.Next() {
		f, err := scanBangouFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) CountBangouFiles(ctx context.Context, bangouID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bangou_files WHERE bangou_id = ?`, bangouID).Scan(&count)
	return count, err
}

// ── Metadata ──

func (s *SQLiteStore) UpsertMetadata(ctx context.Context, m *Metadata) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO metadata (bangou_id, number, title, plot, director, maker, label, series,
		                       actors, genres, cover_url, sample_images, premiered, year, runtime,
		                       rating, review_count, page_url, content_id, provider, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(bangou_id) DO UPDATE SET
		   number=excluded.number,
		   title=excluded.title, plot=excluded.plot, director=excluded.director,
		   maker=excluded.maker, label=excluded.label, series=excluded.series,
		   actors=excluded.actors, genres=excluded.genres, cover_url=excluded.cover_url,
		   sample_images=excluded.sample_images, premiered=excluded.premiered,
		   year=excluded.year, runtime=excluded.runtime, rating=excluded.rating,
		   review_count=excluded.review_count, page_url=excluded.page_url,
		   content_id=excluded.content_id, provider=excluded.provider,
		   updated_at=excluded.updated_at`,
		m.BangouID, m.Number, m.Title, m.Plot, m.Director, m.Maker, m.Label, m.Series,
		m.Actors, m.Genres, m.CoverURL, m.SampleImages, m.Premiered, m.Year, m.Runtime,
		m.Rating, m.ReviewCount, m.PageURL, m.ContentID, m.Provider, time.Now(),
	)
	return err
}

func (s *SQLiteStore) GetMetadataByBangou(ctx context.Context, bangouID int64) (*Metadata, error) {
	m := &Metadata{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, bangou_id, number, title, plot, director, maker, label, series,
		        actors, genres, cover_url, sample_images, premiered, year, runtime,
		        rating, review_count, page_url, content_id, provider,
		        created_at, updated_at
		 FROM metadata WHERE bangou_id = ?`, bangouID,
	).Scan(&m.ID, &m.BangouID, &m.Number, &m.Title, &m.Plot, &m.Director, &m.Maker, &m.Label, &m.Series,
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

// querier is satisfied by both *sql.DB and *sql.Tx.
type querier interface {
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
}

func tableExists(db querier, table string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count)
	return count > 0, err
}

func hasColumn(db querier, table, column string) (bool, error) {
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
