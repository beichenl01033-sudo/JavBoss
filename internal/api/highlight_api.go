package api

import (
	"errors"
	"math"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/video/highlights"
)

func highlightDataDir() string {
	if cfg := common.CurrentConfig(); cfg != nil {
		return filepath.Dir(cfg.DatabasePath)
	}
	return ""
}

func syncHighlightIndex(c *gin.Context, force bool) bool {
	if err := dbpkg.Default().SyncVideoHighlights(c.Request.Context(), highlightDataDir(), force); err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "更新亮点索引失败", "Failed to update highlight index")
		return false
	}
	return true
}

// GET /videos/highlights returns counts for refreshing badges after MPV saves.
func listHighlightCounts(c *gin.Context) {
	if !syncHighlightIndex(c, false) {
		return
	}
	counts, err := dbpkg.Default().VideoHighlightCounts(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "加载亮点数量失败", "Failed to load highlight counts")
		return
	}
	c.JSON(http.StatusOK, gin.H{"counts": counts})
}

func videoHighlightPath(c *gin.Context) (string, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 || highlightDataDir() == "" {
		respondLocalizedError(c, http.StatusBadRequest, "视频 ID 无效", "Invalid video ID")
		return "", false
	}
	video, err := dbpkg.Default().GetVideo(c.Request.Context(), id)
	if err != nil || video == nil {
		respondLocalizedError(c, http.StatusNotFound, "视频不存在", "Video does not exist")
		return "", false
	}
	return highlights.Path(highlightDataDir(), id), true
}

func getVideoHighlights(c *gin.Context) {
	path, ok := videoHighlightPath(c)
	if !ok {
		return
	}
	doc, err := highlights.Read(path)
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取亮点失败", "Failed to read highlights")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, doc)
}

// PATCH /videos/:id/highlights edits/deletes one time point. A null label deletes
// the mark, never the video; revision detects stale Web edits.
func editVideoHighlight(c *gin.Context) {
	path, ok := videoHighlightPath(c)
	if !ok {
		return
	}
	var req struct {
		Time     *float64 `json:"time"`
		Label    *string  `json:"label"`
		Revision string   `json:"revision"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Time == nil || *req.Time < 0 || math.IsNaN(*req.Time) || math.IsInf(*req.Time, 0) {
		respondLocalizedError(c, http.StatusBadRequest, "亮点参数无效", "Invalid highlight request")
		return
	}
	doc, err := highlights.Edit(path, req.Revision, *req.Time, req.Label)
	if errors.Is(err, highlights.ErrConflict) {
		respondLocalizedError(c, http.StatusConflict, "亮点已改变，请刷新后重试", "Highlights changed; refresh and retry")
		return
	}
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "修改亮点失败", "Failed to edit highlight")
		return
	}
	if !syncHighlightIndex(c, true) {
		return
	}
	c.JSON(http.StatusOK, doc)
}
