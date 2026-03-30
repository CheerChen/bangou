package store

import "context"

type Store interface {
	UpsertFile(ctx context.Context, f *SourceFile) error
	SetFileReady(ctx context.Context, path string) error
	GetFileByPath(ctx context.Context, path string) (*SourceFile, error)
	GetFileByID(ctx context.Context, id int64) (*SourceFile, error)
	ListUnreadyFiles(ctx context.Context) ([]SourceFile, error)
	ListUnknownFiles(ctx context.Context) ([]SourceFile, error)
	SetFileIgnored(ctx context.Context, fileID int64) error

	UpsertParsed(ctx context.Context, p *ParsedInfo) error
	GetParsedByFileID(ctx context.Context, fileID int64) (*ParsedInfo, error)

	EnsureGroup(ctx context.Context, number string) (*Group, error)
	GetGroup(ctx context.Context, id int64) (*Group, error)
	ListGroups(ctx context.Context, status string) ([]Group, error)
	UpdateGroupAction(ctx context.Context, id int64, action string) error
	UpdateGroupStatus(ctx context.Context, id int64, status string) error
	GetGroupFiles(ctx context.Context, groupID int64) ([]SourceFile, error)

	CreateOutput(ctx context.Context, o *Output) error
	ListOutputs(ctx context.Context, groupID int64) ([]Output, error)
	SetOutputAlive(ctx context.Context, id int64, alive bool) error
	ListOrphanedOutputs(ctx context.Context) ([]Output, error)

	RecordMergedPart(ctx context.Context, m *MergedPart) error
	GetMergedParts(ctx context.Context, groupID int64) ([]MergedPart, error)

	GetMetadata(ctx context.Context, number string) (*Metadata, error)
	UpsertMetadata(ctx context.Context, m *Metadata) error
	SetMetadataStatus(ctx context.Context, number, status, errors string) error

	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	GetAllSettings(ctx context.Context) (map[string]string, error)

	Close() error
}
