package engine

import (
	"context"
	"log/slog"

	"voice-engine/config"
	"voice-engine/services"
)

// Archiver aliases services.Archiver for async sharded S3 audit uploading.
type Archiver = services.Archiver
type Batch = services.Batch
type File = services.File

// NewArchiver initializes the async S3 audit uploader.
func NewArchiver(ctx context.Context, cfg config.Config, log *slog.Logger) Archiver {
	return services.NewArchiver(ctx, cfg, log)
}
