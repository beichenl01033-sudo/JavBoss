package db

import (
	"context"
	"fmt"
	"strings"

	"javboss/internal/video/ltx"
)

// JavOwnership contains only the requested code and its library status.
type JavOwnership struct {
	Code  string `json:"code"`
	Owned bool   `json:"owned"`
	LTX   bool   `json:"ltx,omitempty"`
}

// LookupJavOwnership counts file locations in undeleted directories,
// including disabled or temporarily offline directories. Metadata alone is not owned.
// Code matching ignores case, spaces, hyphens and underscores, but never prefixes.
func (store *Store) LookupJavOwnership(ctx context.Context, codes []string) ([]JavOwnership, error) {
	separators := strings.NewReplacer("-", "", "_", "", " ", "")
	normalize := func(code string) string {
		return separators.Replace(strings.ToUpper(code))
	}
	items := make([]JavOwnership, 0, len(codes))
	if len(codes) == 0 {
		return items, nil
	}
	keys := make([]string, 0, len(codes))
	for _, code := range codes {
		keys = append(keys, normalize(code))
	}
	var rows []struct {
		Code         string
		Filename     string
		RelativePath string
	}
	err := store.conn.WithContext(ctx).Table("video_location vl").
		Select("jav.code, vl.filename, vl.relative_path").
		Joins("JOIN jav ON jav.id = vl.jav_id").
		Joins("JOIN directory d ON d.id = vl.directory_id").
		Where("COALESCE(d.is_delete, 0) = 0").
		Where("UPPER(REPLACE(REPLACE(REPLACE(code, '-', ''), '_', ''), ' ', '')) IN ?", keys).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("lookup jav ownership: %w", err)
	}
	owned := make(map[string]bool, len(rows))
	marked := make(map[string]bool)
	for _, row := range rows {
		key := normalize(row.Code)
		owned[key] = true
		marked[key] = marked[key] || ltx.Marked(row.Filename) || ltx.Marked(row.RelativePath)
	}
	for _, code := range codes {
		items = append(items, JavOwnership{Code: code, Owned: owned[normalize(code)], LTX: marked[normalize(code)]})
	}
	return items, nil
}
