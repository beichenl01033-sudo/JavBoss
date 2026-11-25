package db

import (
	"gorm.io/gorm"
	"javboss/internal/video/ltx"
)

// LtxFilter is shared by file, work and entity lists. CodesJSON contains only
// normalized codes from a complete catalogue, selected for the requested status.
type LtxFilter struct {
	Local     string
	Catalog   string
	CodesJSON string
}

func firstLtxFilter(filters []LtxFilter) LtxFilter {
	if len(filters) > 0 {
		return filters[0]
	}
	return LtxFilter{}
}

func ltxMarkedSQL(filename, relativePath string) bool {
	if filename == "" {
		filename = relativePath
	}
	return ltx.Marked(filename)
}

func applyLtxLocationFilter(q *gorm.DB, alias string, f LtxFilter, directoryIDs []int64, subdirectory string, workLevel bool) *gorm.DB {
	marked := "ltx_marked(COALESCE(" + alias + ".filename, ''), " + alias + ".relative_path)"
	if f.Local == "with" {
		q = q.Where(marked + " = 1")
	} else if f.Local == "without" {
		if workLevel {
			// A normal copy does not make a work 'without LTX' when another copy
			// in the selected directory scope is already labelled LTX.
			existing := q.Session(&gorm.Session{NewDB: true}).Table("video_location ltx_loc").
				Select("1").Joins("JOIN directory ltx_dir ON ltx_dir.id = ltx_loc.directory_id").
				Where("ltx_loc.jav_id = " + alias + ".jav_id").
				Where(activeDirectoryWhereSQL("ltx_dir")).
				Where("ltx_marked(COALESCE(ltx_loc.filename, ''), ltx_loc.relative_path) = 1")
			existing = applyDirectoryFilter(existing, "ltx_loc", directoryIDs)
			existing = applyLocationSubdirectory(existing, "ltx_loc", subdirectory)
			q = q.Where("NOT EXISTS (?)", existing)
		} else {
			q = q.Where(marked + " = 0")
		}
	}
	if f.Catalog != "" {
		codes := f.CodesJSON
		if codes == "" {
			codes = "[]"
		}
		// json_each uses one bind parameter even for a large catalogue. Unknown
		// (unlinked) files are not classified as 'not listed'.
		code := "UPPER(REPLACE(REPLACE(REPLACE(TRIM(ltx_jav.code), '-', ''), '_', ''), ' ', ''))"
		membership := " IN "
		if f.Catalog == "not_found" {
			membership = " NOT IN "
		}
		q = q.Where("EXISTS (SELECT 1 FROM jav ltx_jav WHERE ltx_jav.id = "+alias+".jav_id AND TRIM(ltx_jav.code) <> '' AND "+code+membership+"(SELECT value FROM json_each(?)))", codes)
	}
	return q
}
