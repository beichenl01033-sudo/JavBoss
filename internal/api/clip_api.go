package api

import (
	"errors"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"

	"javboss/internal/common"
	"javboss/internal/mpv"
	"javboss/internal/util"
)

func listClipTasks(c *gin.Context) {
	items := mpv.ListClipTasks()
	counts := map[string]int{"active": 0, "completed": 0, "failed": 0}
	for _, item := range items {
		switch item.Status {
		case mpv.ClipTaskRunning:
			counts["active"]++
		case mpv.ClipTaskCompleted:
			counts["completed"]++
		case mpv.ClipTaskFailed, mpv.ClipTaskCanceled:
			counts["failed"]++
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "counts": counts})
}

func cancelClipTask(c *gin.Context) {
	id, ok := clipTaskID(c)
	if !ok {
		return
	}
	if err := mpv.CancelClipTask(id); err != nil {
		if errors.Is(err, mpv.ErrClipTaskNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "剪辑任务不存在", "Clip task not found")
			return
		}
		respondLocalizedError(c, http.StatusConflict, "该剪辑任务当前不能取消", "The clip task cannot be canceled")
		return
	}
	c.Status(http.StatusNoContent)
}

func deleteClipTask(c *gin.Context) {
	id, ok := clipTaskID(c)
	if !ok {
		return
	}
	if err := mpv.DeleteClipTask(id); err != nil {
		if errors.Is(err, mpv.ErrClipTaskNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "剪辑任务不存在", "Clip task not found")
			return
		}
		respondLocalizedError(c, http.StatusConflict, "请先等待剪辑结束或取消任务", "Wait for the clip task to finish or cancel it first")
		return
	}
	c.Status(http.StatusNoContent)
}

func revealClipTask(c *gin.Context) {
	if isRemoteRequest(c.Request.RemoteAddr) {
		respondLocalizedError(c, http.StatusForbidden, "通过局域网访问时无法打开剪辑位置", "Cannot reveal clip locations when accessing over the local network")
		return
	}
	if common.ContainerMode() {
		respondLocalizedError(c, http.StatusNotImplemented, "当前部署模式已禁用打开剪辑位置", "Desktop folder revealing is disabled")
		return
	}
	id, ok := clipTaskID(c)
	if !ok {
		return
	}
	path, err := mpv.ClipTaskOutputPath(id)
	if err != nil {
		if errors.Is(err, mpv.ErrClipTaskNotFound) {
			respondLocalizedError(c, http.StatusNotFound, "剪辑任务不存在", "Clip task not found")
			return
		}
		respondLocalizedError(c, http.StatusConflict, "剪辑文件尚未生成", "The clip output is not ready")
		return
	}
	if _, err := os.Stat(path); err != nil {
		respondLocalizedError(c, http.StatusNotFound, "剪辑文件不存在", "Clip output does not exist")
		return
	}
	if err := util.RevealFile(path); err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "打开剪辑位置失败", "Failed to reveal clip output")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func clipTaskID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "剪辑任务编号无效", "Invalid clip task ID")
		return 0, false
	}
	return id, true
}
