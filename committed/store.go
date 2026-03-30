package committed

import "context"

type Store interface {
	CreateOutput(ctx context.Context, o *Output) error
	DeleteOutput(ctx context.Context, id int64) error
	ListOutputsByNumber(ctx context.Context, number string) ([]Output, error)
	ListAllOutputs(ctx context.Context) ([]Output, error)
	SetOutputAlive(ctx context.Context, id int64, alive bool) error
	ListOrphanedOutputs(ctx context.Context) ([]Output, error)
	IsCommitted(ctx context.Context, number string) (bool, error)

	RecordMergedParts(ctx context.Context, parts []MergedPart) error
	GetMergedParts(ctx context.Context, number string) ([]MergedPart, error)

	UpsertMetadata(ctx context.Context, m *Metadata) error
	GetMetadata(ctx context.Context, number string) (*Metadata, error)

	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	GetAllSettings(ctx context.Context) (map[string]string, error)

	Close() error
}
