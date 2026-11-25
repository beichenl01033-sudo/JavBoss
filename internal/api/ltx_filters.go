package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	dbpkg "javboss/internal/db"
	"javboss/internal/ltxcatalog"
)

// ltx_local=with|without selects labelled local files (works use scoped absence).
// ltx_catalog=confirmed|pending|not_found uses the last complete cached catalogue.
// Omission means all. Without a complete catalogue, return 409, never false negatives.
func parseLtxFilter(c *gin.Context) (dbpkg.LtxFilter, bool) {
	f := dbpkg.LtxFilter{Local: strings.TrimSpace(c.Query("ltx_local")), Catalog: strings.TrimSpace(c.Query("ltx_catalog"))}
	if (f.Local != "" && f.Local != "with" && f.Local != "without") ||
		(f.Catalog != "" && f.Catalog != "confirmed" && f.Catalog != "pending" && f.Catalog != "not_found") {
		respondLocalizedError(c, http.StatusBadRequest, "LTX 筛选参数无效", "Invalid LTX filter")
		return f, false
	}
	if f.Catalog == "" {
		return f, true
	}
	snapshot, err := ltxcatalog.Load(highlightDataDir())
	if err != nil || !snapshot.Complete {
		respondLocalizedError(c, http.StatusConflict, "请先通过浏览器扩展完整同步 LTX 目录，再使用新站收录筛选", "Sync the complete LTX catalogue with the browser extension before filtering")
		return f, false
	}
	codes := make([]string, 0, snapshot.Total)
	for _, item := range snapshot.Items {
		if f.Catalog == "not_found" || (f.Catalog == "pending") == item.Pending {
			codes = append(codes, ltxcatalog.Key(item.Code))
		}
	}
	raw, err := json.Marshal(codes)
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取 LTX 目录失败", "Failed to read LTX catalogue")
		return f, false
	}
	f.CodesJSON = string(raw)
	return f, true
}
