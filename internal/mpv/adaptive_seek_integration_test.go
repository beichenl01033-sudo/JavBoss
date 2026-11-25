package mpv

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAdaptiveSeekWithRealMPVKeyEvents(t *testing.T) {
	if os.Getenv("JAVBOSS_TEST_MPV") == "" {
		t.Skip("set JAVBOSS_TEST_MPV for real seek key events")
	}
	if os.Getenv("JAVBOSS_ADAPTIVE_SEEK_HELPER") == "" {
		// The binary resolver caches its first MPV_PATH for the process. Keep
		// this custom binary and hotkey provider isolated from other tests.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(ctx, executable, "-test.run=^TestAdaptiveSeekWithRealMPVKeyEvents$")
		command.Env = append(os.Environ(), "JAVBOSS_ADAPTIVE_SEEK_HELPER=1")
		if output, err := command.CombinedOutput(); err != nil || !strings.Contains(string(output), "ADAPTIVE_SEEK_RUNTIME_PASSED") {
			t.Fatalf("adaptive seek integration: %v\n%s", err, output)
		}
		return
	}
	dir := t.TempDir()
	t.Setenv("MPV_PATH", os.Getenv("JAVBOSS_TEST_MPV"))
	source, _ := filepath.Abs("../../modernz")
	t.Setenv(modernZEnvDir, source)
	previous := playerConfigProvider
	SetPlayerConfigProvider(func() (map[string]string, error) {
		return map[string]string{playerHotkeysConfigKey: `[{"key":"ArrowRight","action":"seek","amount":1},{"key":"ArrowLeft","action":"seek","amount":-1}]`}, nil
	})
	t.Cleanup(func() { SetPlayerConfigProvider(previous) })
	ffmpeg := os.Getenv("JAVBOSS_TEST_FFMPEG")
	if ffmpeg == "" {
		ffmpeg, _ = filepath.Abs("../../internal/bin/ffmpeg")
	}
	if _, err := os.Stat(ffmpeg); err != nil {
		t.Skip("set JAVBOSS_TEST_FFMPEG for the long-GOP video fixture")
	}
	media := filepath.Join(dir, "seek.mp4")
	// Ten-second GOPs expose the difference between logical seek targets and
	// time-pos rounded to keyframes; no user video is read or rewritten.
	if output, err := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=160x90:rate=10", "-t", "120", "-an", "-c:v", "libx264", "-preset", "ultrafast", "-g", "100", "-keyint_min", "100", "-sc_threshold", "0", media).CombinedOutput(); err != nil {
		t.Fatalf("video fixture: %v %s", err, output)
	}
	process, endpoint, err := buildCommandWithIPC("", PlayOptions{DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	process.Args = append(process.Args, "--vo=null", "--ao=null", "--force-window=no", "--terminal=no", "--pause=yes", "--save-position-on-quit=no")
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = process.Process.Kill(); _ = process.Wait() })
	if err := waitForIPCReady(endpoint); err != nil {
		t.Fatal(err)
	}
	command := func(args ...any) {
		t.Helper()
		if err := runIPCCommand(endpoint, args); err != nil {
			t.Fatal(err)
		}
	}
	command("loadfile", media, "replace")
	waitAdaptiveSeekProperty(t, endpoint, "duration", float64(120))
	command("set_property", "pause", true)
	command("seek", 10, "absolute+exact")
	waitAdaptiveSeekProperty(t, endpoint, "seeking", false)
	position := func(want float64) {
		t.Helper()
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			if at, ok := readAdaptiveSeekProperty(t, endpoint, "time-pos").(float64); ok && math.Abs(at-want) < 0.1 {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("time-pos=%v want %v", readAdaptiveSeekProperty(t, endpoint, "time-pos"), want)
	}
	position(10)
	status, ok := readAdaptiveSeekProperty(t, endpoint, "user-data/javboss/decoder-status").(map[string]any)
	if !ok || status["active"] != "no" || status["codec"] != "h264" {
		t.Fatalf("incorrect actual decoder status: %v", status)
	}
	command("keydown", "RIGHT")
	// A physical tap is committed on keyup, not as repeated expensive seeks.
	time.Sleep(50 * time.Millisecond)
	position(10)
	command("keyup", "RIGHT")
	position(11)
	command("keydown", "LEFT")
	time.Sleep(50 * time.Millisecond)
	command("keyup", "LEFT")
	position(10)
	command("keydown", "RIGHT")
	time.Sleep(650 * time.Millisecond)
	preview, ok := readAdaptiveSeekProperty(t, endpoint, "user-data/javboss/seek-preview").(map[string]any)
	if !ok {
		t.Fatalf("missing held-key target (complex binding arg/event lost): %v", preview)
	}
	target, ok := preview["target"].(float64)
	if !ok || target <= 12 {
		t.Fatalf("repeat did not advance target: %v", preview)
	}
	command("keyup", "RIGHT")
	// Readback races the very last autorepeat before release; use the actual
	// last coarse/precise target published by the controller to verify cleanup.
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if readAdaptiveSeekProperty(t, endpoint, "user-data/javboss/seek-preview") == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	waitAdaptiveSeekProperty(t, endpoint, "seeking", false)
	at := readAdaptiveSeekProperty(t, endpoint, "time-pos").(float64)
	if at < target || at > target+3 {
		t.Fatalf("release position=%v preview=%v", at, target)
	}
	time.Sleep(250 * time.Millisecond)
	position(at)
	if readAdaptiveSeekProperty(t, endpoint, "pause") != true {
		t.Fatal("seek unpaused playback")
	}
	command("keypress", "RIGHT")
	position(at + 1)
	command("set_property", "hwdec", "auto")
	// Copy-back hardware decoding can work even with vo=null. Compare actual
	// properties instead of assuming a headless process always uses software.
	decoder := func(wantSoftware bool) {
		t.Helper()
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			status, _ = readAdaptiveSeekProperty(t, endpoint, "user-data/javboss/decoder-status").(map[string]any)
			active := readAdaptiveSeekProperty(t, endpoint, "hwdec-current")
			if status["requested"] == "auto" && status["codec"] == "h264" && status["active"] == active && active != nil && (!wantSoftware || active == "no") {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("incorrect decoder status (software=%v): %v", wantSoftware, status)
	}
	decoder(false)
	command("set_property", "hwdec", "no")
	command("set_property", "hwdec-codecs", "vp9") // h264 is deliberately not eligible
	command("set_property", "hwdec", "auto")
	// Decoder reconfiguration may be deferred while paused. Reopen the video
	// as a real automatic-mode playback rather than inspecting an idle decoder.
	command("loadfile", media, "replace")
	command("set_property", "pause", false)
	decoder(true)
	command("set_property", "pause", true)
	command("keydown", "RIGHT")
	time.Sleep(250 * time.Millisecond)
	command("stop")
	command("keyup", "RIGHT")
	waitAdaptiveSeekProperty(t, endpoint, "user-data/javboss/seek-preview", nil)
	fmt.Println("ADAPTIVE_SEEK_RUNTIME_PASSED")
}

func readAdaptiveSeekProperty(t *testing.T, endpoint, name string) any {
	t.Helper()
	raw, err := queryIPCCommand(endpoint, []any{"get_property", name})
	if err != nil {
		if isIPCResponseError(err) {
			return nil
		} // e.g. an unloaded decoder
		t.Fatal(err)
	}
	var value any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatal(err)
		}
	}
	return value
}

func waitAdaptiveSeekProperty(t *testing.T, endpoint, name string, want any) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if readAdaptiveSeekProperty(t, endpoint, name) == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s=%v want %v", name, readAdaptiveSeekProperty(t, endpoint, name), want)
}
