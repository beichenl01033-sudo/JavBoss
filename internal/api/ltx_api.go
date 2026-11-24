package api

import (
	"github.com/gin-gonic/gin"
	"javboss/internal/ltxcatalog"
	"net/http"
)

// GET /ltx/catalog returns only the last complete catalogue and its refresh time.
func getLtxCatalog(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	s, err := ltxcatalog.Load(highlightDataDir())
	if err != nil {
		respondLocalizedError(c, 500, "读取 LTX 目录缓存失败", "Failed to read LTX catalogue")
		return
	}
	c.JSON(200, s)
}

// POST /extension/ltx/catalog imports a complete public index. No media URLs,
// local paths or client supplied cache timestamps are accepted or persisted.
func importLtxCatalog(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ltxcatalog.MaxBytes)
	var s ltxcatalog.Snapshot
	if err := c.ShouldBindJSON(&s); err != nil || ltxcatalog.Validate(s) != nil {
		respondLocalizedError(c, 400, "LTX 目录不完整或条目重复，保留原缓存", "Incomplete or duplicate LTX catalogue; previous cache retained")
		return
	}
	saved, err := ltxcatalog.Save(highlightDataDir(), s)
	if err != nil {
		respondLocalizedError(c, 500, "保存 LTX 目录失败，保留原缓存", "Failed to save LTX catalogue; previous cache retained")
		return
	}
	c.JSON(200, gin.H{"total": saved.Total, "updated_at": saved.UpdatedAt})
}
