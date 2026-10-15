package mpv

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestClipTaskHelperProcess(t *testing.T) {
	mode := os.Getenv("GO_WANT_CLIP_TASK_HELPER")
	if mode == "" {
		return
	}
	if mode == "slow" {
		time.Sleep(10 * time.Second)
		return
	}
	args := os.Args
	separator := -1
	for index, value := range args {
		if value == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(args) {
		os.Exit(2)
	}
	ffmpegArgs := args[separator+1:]
	var source string
	for index, value := range ffmpegArgs {
		if value == "-i" && index+1 < len(ffmpegArgs) {
			source = ffmpegArgs[index+1]
			break
		}
	}
	output := ffmpegArgs[len(ffmpegArgs)-1]
	input, err := os.Open(source)
	if err != nil {
		os.Exit(3)
	}
	defer input.Close()
	target, err := os.Create(output)
	if err != nil {
		os.Exit(4)
	}
	if _, err := io.Copy(target, input); err != nil {
		target.Close()
		os.Exit(5)
	}
	if err := target.Close(); err != nil {
		os.Exit(6)
	}
	fmt.Println("out_time_us=1000000")
	fmt.Println("progress=end")
}

func TestClipTaskCompletesOutsideMPVAndPreservesModifiedTime(t *testing.T) {
	manager := newClipTaskManager(
		func() (string, error) { return "clip-helper", nil },
		clipHelperCommand("complete"),
	)
	t.Cleanup(manager.shutdown)

	directory := t.TempDir()
	source := filepath.Join(directory, "source video.mp4")
	if err := os.WriteFile(source, []byte("video"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	modifiedAt := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(source, modifiedAt, modifiedAt); err != nil {
		t.Fatalf("set source time: %v", err)
	}

	task, err := manager.submit(clipRequest{Source: source, StartSeconds: 1, EndSeconds: 3})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	completed := waitForClipTask(t, manager, task.ID)
	if completed.Status != ClipTaskCompleted || completed.Progress != 100 {
		t.Fatalf("completed task = %+v", completed)
	}
	if completed.OutputFilename != "source video.javboss-clip-00h00m01s-00h00m03s.mp4" {
		t.Fatalf("output filename = %q", completed.OutputFilename)
	}
	outputInfo, err := os.Stat(completed.OutputPath)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if !outputInfo.ModTime().Equal(modifiedAt) {
		t.Fatalf("output modified time = %v, want %v", outputInfo.ModTime(), modifiedAt)
	}
	if _, err := os.Stat(completed.OutputPath + ".part.mp4"); !os.IsNotExist(err) {
		t.Fatalf("temporary output still exists: %v", err)
	}
}

func TestClipTaskCanBeCanceled(t *testing.T) {
	manager := newClipTaskManager(
		func() (string, error) { return "clip-helper", nil },
		clipHelperCommand("slow"),
	)
	t.Cleanup(manager.shutdown)
	source := filepath.Join(t.TempDir(), "source.mp4")
	if err := os.WriteFile(source, []byte("video"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	task, err := manager.submit(clipRequest{Source: source, StartSeconds: 1, EndSeconds: 3})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if err := manager.cancel(task.ID); err != nil {
		t.Fatalf("cancel task: %v", err)
	}
	canceled := waitForClipTask(t, manager, task.ID)
	if canceled.Status != ClipTaskCanceled {
		t.Fatalf("canceled task = %+v", canceled)
	}
}

func TestClipTaskStagesNetworkOutputLocallyBeforeCopyingBack(t *testing.T) {
	manager := newClipTaskManager(
		func() (string, error) { return "clip-helper", nil },
		clipHelperCommand("complete"),
	)
	manager.shouldStage = func(string) bool { return true }
	manager.localTempRoot = t.TempDir()
	t.Cleanup(manager.shutdown)

	sourceDirectory := t.TempDir()
	source := filepath.Join(sourceDirectory, "nas source.mp4")
	if err := os.WriteFile(source, []byte("network-video"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	modifiedAt := time.Date(2019, 5, 6, 7, 8, 9, 0, time.UTC)
	if err := os.Chtimes(source, modifiedAt, modifiedAt); err != nil {
		t.Fatalf("set source time: %v", err)
	}

	task, err := manager.submit(clipRequest{Source: source, StartSeconds: 2, EndSeconds: 4})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if !task.NetworkOptimized || task.Phase != ClipPhaseReadingNetwork {
		t.Fatalf("network task did not enable local staging: %+v", task)
	}
	completed := waitForClipTask(t, manager, task.ID)
	if completed.Status != ClipTaskCompleted || completed.Phase != ClipPhaseCompleted {
		t.Fatalf("completed staged task = %+v", completed)
	}
	content, err := os.ReadFile(completed.OutputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(content) != "network-video" {
		t.Fatalf("output content = %q", content)
	}
	info, err := os.Stat(completed.OutputPath)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if !info.ModTime().Equal(modifiedAt) {
		t.Fatalf("output modified time = %v, want %v", info.ModTime(), modifiedAt)
	}
	matches, err := filepath.Glob(filepath.Join(manager.localTempRoot, "javboss", "clip-tasks", "clip-*"))
	if err != nil {
		t.Fatalf("glob staging files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("staging files were not cleaned up: %v", matches)
	}
}

func clipHelperCommand(mode string) clipCommandContext {
	return func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		commandArgs := append([]string{"-test.run=^TestClipTaskHelperProcess$", "--"}, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], commandArgs...)
		cmd.Env = append(os.Environ(), "GO_WANT_CLIP_TASK_HELPER="+mode)
		return cmd
	}
}

func waitForClipTask(t *testing.T, manager *clipTaskManager, id int64) ClipTask {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, item := range manager.list() {
			if item.ID == id && item.Status != ClipTaskRunning {
				return item
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("clip task %d did not finish", id)
	return ClipTask{}
}
