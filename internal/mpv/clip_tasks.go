package mpv

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/util"
)

const (
	ClipTaskRunning   = "running"
	ClipTaskCompleted = "completed"
	ClipTaskFailed    = "failed"
	ClipTaskCanceled  = "canceled"
)

var (
	ErrClipTaskNotFound      = errors.New("clip task not found")
	ErrClipTaskNotCancelable = errors.New("clip task cannot be canceled")
	ErrClipTaskNotDeletable  = errors.New("clip task cannot be deleted")
)

type ClipTask struct {
	ID             int64      `json:"id"`
	SourcePath     string     `json:"source_path"`
	SourceFilename string     `json:"source_filename"`
	OutputPath     string     `json:"output_path"`
	OutputFilename string     `json:"output_filename"`
	StartSeconds   float64    `json:"start_seconds"`
	EndSeconds     float64    `json:"end_seconds"`
	Progress       float64    `json:"progress"`
	Status         string     `json:"status"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
}

type clipRequest struct {
	RequestID    string  `json:"request_id"`
	Source       string  `json:"source"`
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
}

type clipTaskState struct {
	task   ClipTask
	cancel context.CancelFunc
}

type clipCommandContext func(context.Context, string, ...string) *exec.Cmd

type clipTaskManager struct {
	mu             sync.Mutex
	tasks          map[int64]*clipTaskState
	order          []int64
	nextID         int64
	closed         bool
	resolveFFmpeg  func() (string, error)
	commandContext clipCommandContext
	wg             sync.WaitGroup
}

var defaultClipTasks = newClipTaskManager(util.ResolveFFmpegPath, util.BackgroundCommandContext)

func newClipTaskManager(resolveFFmpeg func() (string, error), commandContext clipCommandContext) *clipTaskManager {
	return &clipTaskManager{
		tasks:          make(map[int64]*clipTaskState),
		resolveFFmpeg:  resolveFFmpeg,
		commandContext: commandContext,
	}
}

func submitClipRequestJSON(raw string) error {
	var request clipRequest
	if err := json.Unmarshal([]byte(raw), &request); err != nil {
		return fmt.Errorf("decode clip request: %w", err)
	}
	_, err := defaultClipTasks.submit(request)
	return err
}

func ListClipTasks() []ClipTask {
	return defaultClipTasks.list()
}

func CancelClipTask(id int64) error {
	return defaultClipTasks.cancel(id)
}

func DeleteClipTask(id int64) error {
	return defaultClipTasks.delete(id)
}

func ClipTaskOutputPath(id int64) (string, error) {
	return defaultClipTasks.outputPath(id)
}

func (m *clipTaskManager) submit(request clipRequest) (ClipTask, error) {
	source := filepath.Clean(strings.TrimSpace(request.Source))
	if source == "." || source == "" {
		return ClipTask{}, errors.New("clip source path is empty")
	}
	if math.IsNaN(request.StartSeconds) || math.IsInf(request.StartSeconds, 0) ||
		math.IsNaN(request.EndSeconds) || math.IsInf(request.EndSeconds, 0) ||
		request.StartSeconds < 0 || request.EndSeconds <= request.StartSeconds {
		return ClipTask{}, errors.New("clip range is invalid")
	}
	info, err := os.Stat(source)
	if err != nil {
		return ClipTask{}, fmt.Errorf("stat clip source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return ClipTask{}, errors.New("clip source is not a regular file")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ClipTask{}, errors.New("clip task manager is shutting down")
	}
	outputPath := m.nextOutputPathLocked(source, request.StartSeconds, request.EndSeconds)
	m.nextID++
	now := time.Now().UTC()
	ctx, cancel := context.WithCancel(context.Background())
	state := &clipTaskState{
		task: ClipTask{
			ID:             m.nextID,
			SourcePath:     source,
			SourceFilename: filepath.Base(source),
			OutputPath:     outputPath,
			OutputFilename: filepath.Base(outputPath),
			StartSeconds:   request.StartSeconds,
			EndSeconds:     request.EndSeconds,
			Status:         ClipTaskRunning,
			CreatedAt:      now,
			StartedAt:      &now,
		},
		cancel: cancel,
	}
	m.tasks[state.task.ID] = state
	m.order = append([]int64{state.task.ID}, m.order...)
	m.wg.Add(1)
	go m.run(ctx, state.task.ID)
	return state.task, nil
}

func (m *clipTaskManager) list() []ClipTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]ClipTask, 0, len(m.order))
	for _, id := range m.order {
		if state := m.tasks[id]; state != nil {
			items = append(items, state.task)
		}
	}
	return items
}

func (m *clipTaskManager) cancel(id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.tasks[id]
	if state == nil {
		return ErrClipTaskNotFound
	}
	if state.task.Status != ClipTaskRunning || state.cancel == nil {
		return ErrClipTaskNotCancelable
	}
	state.cancel()
	return nil
}

func (m *clipTaskManager) delete(id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.tasks[id]
	if state == nil {
		return ErrClipTaskNotFound
	}
	if state.task.Status == ClipTaskRunning {
		return ErrClipTaskNotDeletable
	}
	delete(m.tasks, id)
	for index, itemID := range m.order {
		if itemID == id {
			m.order = append(m.order[:index], m.order[index+1:]...)
			break
		}
	}
	return nil
}

func (m *clipTaskManager) outputPath(id int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.tasks[id]
	if state == nil {
		return "", ErrClipTaskNotFound
	}
	if state.task.Status != ClipTaskCompleted {
		return "", errors.New("clip output is not ready")
	}
	return state.task.OutputPath, nil
}

func (m *clipTaskManager) nextOutputPathLocked(source string, startSeconds, endSeconds float64) string {
	directory := filepath.Dir(source)
	extension := filepath.Ext(source)
	if extension == "" {
		extension = ".mkv"
	}
	stem := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	suffix := ".javboss-clip-" + clipFilenameTime(startSeconds) + "-" + clipFilenameTime(endSeconds)
	reserved := make(map[string]struct{}, len(m.tasks))
	for _, state := range m.tasks {
		reserved[filepath.Clean(state.task.OutputPath)] = struct{}{}
	}
	for sequence := 1; ; sequence++ {
		sequenceSuffix := ""
		if sequence > 1 {
			sequenceSuffix = fmt.Sprintf("-%02d", sequence)
		}
		candidate := filepath.Join(directory, stem+suffix+sequenceSuffix+extension)
		if _, used := reserved[filepath.Clean(candidate)]; used {
			continue
		}
		if _, err := os.Stat(candidate); err != nil {
			return candidate
		}
	}
}

func clipFilenameTime(seconds float64) string {
	total := int64(math.Floor(math.Max(0, seconds) + 0.5))
	return fmt.Sprintf("%02dh%02dm%02ds", total/3600, (total%3600)/60, total%60)
}

func (m *clipTaskManager) run(ctx context.Context, id int64) {
	defer m.wg.Done()
	m.mu.Lock()
	state := m.tasks[id]
	if state == nil {
		m.mu.Unlock()
		return
	}
	task := state.task
	m.mu.Unlock()

	ffmpegPath, err := m.resolveFFmpeg()
	if err != nil {
		m.finish(id, ClipTaskFailed, err)
		return
	}
	extension := filepath.Ext(task.OutputPath)
	tempPath := task.OutputPath + ".part" + extension
	_ = os.Remove(tempPath)
	duration := task.EndSeconds - task.StartSeconds
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-ss", strconv.FormatFloat(task.StartSeconds, 'f', -1, 64),
		"-i", task.SourcePath,
		"-t", strconv.FormatFloat(duration, 'f', -1, 64),
		"-map", "0",
		"-c", "copy",
		"-avoid_negative_ts", "make_zero",
		"-progress", "pipe:1",
		"-nostats",
		"-y", tempPath,
	}
	cmd := m.commandContext(ctx, ffmpegPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.finish(id, ClipTaskFailed, fmt.Errorf("read ffmpeg progress: %w", err))
		return
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	logging.Info("clip task start: id=%d source=%s output=%s", id, task.SourcePath, task.OutputPath)
	if err := cmd.Start(); err != nil {
		if ctx.Err() != nil {
			m.finish(id, ClipTaskCanceled, nil)
			return
		}
		m.finish(id, ClipTaskFailed, fmt.Errorf("start ffmpeg: %w", err))
		return
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			continue
		}
		if key == "out_time_us" || key == "out_time_ms" {
			microseconds, parseErr := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if parseErr == nil && duration > 0 {
				m.updateProgress(id, math.Min(99.9, float64(microseconds)/1_000_000/duration*100))
			}
		}
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		_ = os.Remove(tempPath)
		m.finish(id, ClipTaskCanceled, nil)
		return
	}
	if scanErr := scanner.Err(); scanErr != nil && waitErr == nil {
		waitErr = scanErr
	}
	if waitErr != nil {
		_ = os.Remove(tempPath)
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = waitErr.Error()
		}
		m.finish(id, ClipTaskFailed, errors.New(detail))
		return
	}
	sourceInfo, err := os.Stat(task.SourcePath)
	if err != nil {
		_ = os.Remove(tempPath)
		m.finish(id, ClipTaskFailed, fmt.Errorf("stat clip source: %w", err))
		return
	}
	if err := os.Chtimes(tempPath, sourceInfo.ModTime(), sourceInfo.ModTime()); err != nil {
		_ = os.Remove(tempPath)
		m.finish(id, ClipTaskFailed, fmt.Errorf("copy source modified time: %w", err))
		return
	}
	if err := os.Rename(tempPath, task.OutputPath); err != nil {
		_ = os.Remove(tempPath)
		m.finish(id, ClipTaskFailed, fmt.Errorf("save clip output: %w", err))
		return
	}
	m.finish(id, ClipTaskCompleted, nil)
}

func (m *clipTaskManager) updateProgress(id int64, progress float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state := m.tasks[id]; state != nil && state.task.Status == ClipTaskRunning {
		state.task.Progress = math.Max(state.task.Progress, math.Max(0, progress))
	}
}

func (m *clipTaskManager) finish(id int64, status string, taskErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.tasks[id]
	if state == nil {
		return
	}
	now := time.Now().UTC()
	state.task.Status = status
	state.task.FinishedAt = &now
	state.cancel = nil
	if status == ClipTaskCompleted {
		state.task.Progress = 100
	}
	if taskErr != nil {
		state.task.ErrorMessage = taskErr.Error()
		logging.Error("clip task failed: id=%d err=%v", id, taskErr)
	} else {
		logging.Info("clip task finished: id=%d status=%s", id, status)
	}
}

func (m *clipTaskManager) shutdown() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		m.wg.Wait()
		return
	}
	m.closed = true
	for _, state := range m.tasks {
		if state.cancel != nil {
			state.cancel()
		}
	}
	m.mu.Unlock()
	m.wg.Wait()
}
