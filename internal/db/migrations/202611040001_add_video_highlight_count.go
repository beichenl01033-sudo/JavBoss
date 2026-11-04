package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("202611040001_add_video_highlight_count.go", addVideoHighlightCount, irreversibleMigration)
}

func addVideoHighlightCount(ctx context.Context, tx *sql.Tx) error {
	if err := addColumnIfMissing(ctx, tx, "video", "highlight_count", "integer NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return execDB(ctx, tx, "CREATE INDEX IF NOT EXISTS idx_video_highlight_count ON video(highlight_count)")
}
