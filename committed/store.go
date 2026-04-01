package committed

import "context"

type Store interface {
	// Pipelines
	CreatePipeline(ctx context.Context, p *Pipeline) (int64, error)
	DeletePipeline(ctx context.Context, id int64) error
	GetPipeline(ctx context.Context, id int64) (*Pipeline, error)
	ListPipelines(ctx context.Context) ([]Pipeline, error)

	// Provider configs (global, shared across pipelines)
	GetProviderConfig(ctx context.Context, provider string) (string, error)
	SetProviderConfig(ctx context.Context, provider, configJSON string) error
	ListProviderConfigs(ctx context.Context) ([]ProviderConfig, error)

	// Outputs
	CreateOutput(ctx context.Context, o *Output) error
	GetOutputByID(ctx context.Context, id int64) (*Output, error)
	DeleteOutput(ctx context.Context, id int64) error
	ListOutputsByPipeline(ctx context.Context, pipelineID int64, limit, offset int, sort, order string) ([]Output, int, error)
	ListOutputGroupsByPipeline(ctx context.Context, pipelineID int64, limit, offset int, sort, order string) ([]OutputGroup, int, error)
	ListAllOutputs(ctx context.Context) ([]Output, error)
	SetOutputAlive(ctx context.Context, id int64, alive bool) error
	SetOutputLinkType(ctx context.Context, id int64, linkType string) error
	SetOutputSrcPath(ctx context.Context, id int64, srcPath string) error
	SetOutputMedia(ctx context.Context, id int64, fileSize int64, resolution, videoCodec, audioCodec, duration, bitrate string) error
	ListOrphanedOutputs(ctx context.Context) ([]Output, error)
	IsCommitted(ctx context.Context, number string) (bool, error)

	// Merged parts
	RecordMergedParts(ctx context.Context, parts []MergedPart) error
	GetMergedParts(ctx context.Context, number string) ([]MergedPart, error)

	// Metadata
	UpsertMetadata(ctx context.Context, m *Metadata) error
	GetMetadata(ctx context.Context, number string) (*Metadata, error)

	// Legacy settings (kept for migration, minimal use going forward)
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	GetAllSettings(ctx context.Context) (map[string]string, error)

	Close() error
}
