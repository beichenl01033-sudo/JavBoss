package client

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"javboss/internal/mpv"
	"javboss/internal/util"
)

func (c *Client) handleClipTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		respondJSONError(w, http.StatusMethodNotAllowed, "请求方法不支持", "Method not allowed")
		return
	}
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
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "counts": counts})
}

func (c *Client) handleClipTaskAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/clips/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		respondJSONError(w, http.StatusBadRequest, "剪辑任务编号无效", "Invalid clip task ID")
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		respondJSONError(w, http.StatusBadRequest, "剪辑任务编号无效", "Invalid clip task ID")
		return
	}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		if err := mpv.DeleteClipTask(id); err != nil {
			writeClipTaskError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if len(parts) != 2 || r.Method != http.MethodPost {
		respondJSONError(w, http.StatusMethodNotAllowed, "请求方法不支持", "Method not allowed")
		return
	}
	switch parts[1] {
	case "cancel":
		if err := mpv.CancelClipTask(id); err != nil {
			writeClipTaskError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case "reveal":
		path, err := mpv.ClipTaskOutputPath(id)
		if err != nil {
			writeClipTaskError(w, err)
			return
		}
		if _, err := os.Stat(path); err != nil {
			respondJSONError(w, http.StatusNotFound, "剪辑文件不存在", "Clip output does not exist")
			return
		}
		if err := util.RevealFile(path); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "打开剪辑位置失败", "Failed to reveal clip output")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		respondJSONError(w, http.StatusNotFound, "剪辑任务操作不存在", "Clip task action not found")
	}
}

func writeClipTaskError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mpv.ErrClipTaskNotFound):
		respondJSONError(w, http.StatusNotFound, "剪辑任务不存在", "Clip task not found")
	case errors.Is(err, mpv.ErrClipTaskNotCancelable):
		respondJSONError(w, http.StatusConflict, "该剪辑任务当前不能取消", "The clip task cannot be canceled")
	case errors.Is(err, mpv.ErrClipTaskNotDeletable):
		respondJSONError(w, http.StatusConflict, "请先等待剪辑结束或取消任务", "Wait for the clip task to finish or cancel it first")
	default:
		respondJSONError(w, http.StatusConflict, "剪辑文件尚未生成", "The clip output is not ready")
	}
}
