//go:build !windows

package mpv

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSidebarCollapseKeepsBottomControls(t *testing.T) {
	path := os.Getenv("JAVBOSS_TEST_MPV_GUI")
	if path == "" {
		t.Skip("set JAVBOSS_TEST_MPV_GUI to verify real OSC visibility")
	}
	dir, err := os.MkdirTemp("", "mpv-bar-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sourceDir, err := findModernZSourceDir()
	if err != nil {
		t.Fatal(err)
	}
	image, err := filepath.Abs("../../web/public/icon-192.png")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := filepath.Join(dir, "ipc.sock")
	cmd := exec.Command(path, "--no-config", "--load-scripts=no", "--ao=null", "--geometry=640x360", "--keepaspect-window=no",
		"--input-ipc-server="+endpoint, "--image-display-duration=inf", "--save-position-on-quit=no",
		"--script="+filepath.Join(sourceDir, "modernz.lua"), "--script="+filepath.Join(sourceDir, "playlist_sidebar.lua"),
		"--script-opts=playlist_sidebar-auto_hide_single=no,modernz-hidetimeout=1500,modernz-fadeduration=100", image)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = runIPCCommand(endpoint, []any{"quit"})
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	})
	if err := waitForIPCReady(endpoint); err != nil {
		t.Fatal(err)
	}
	waitMPVTestProperty(t, endpoint, "user-data/osc/visibility", "always")
	active := func() bool {
		bindings, _ := readMPVTestProperty(t, endpoint, "input-bindings").([]any)
		for _, raw := range bindings {
			binding, _ := raw.(map[string]any)
			if binding["owner"] == "modernz" && binding["section"] == "input" {
				priority, _ := binding["priority"].(float64)
				if priority >= 0 {
					return true
				}
			}
		}
		return false
	}
	deadline := time.Now().Add(3 * time.Second)
	for !active() {
		if time.Now().After(deadline) {
			t.Fatal("OSC did not initialize")
		}
		time.Sleep(20 * time.Millisecond)
	}
	width := readMPVTestProperty(t, endpoint, "osd-width")
	height := readMPVTestProperty(t, endpoint, "osd-height")
	if err := runIPCCommand(endpoint, []any{"script-message-to", "playlist_sidebar", "toggle"}); err != nil {
		t.Fatal(err)
	}
	waitMPVTestProperty(t, endpoint, "user-data/javboss/playlist-sidebar-width", float64(0))
	waitMPVTestProperty(t, endpoint, "user-data/osc/visibility", "auto")
	// Longer than the configured fade-out, shorter than normal auto-hide.
	time.Sleep(400 * time.Millisecond)
	if !active() {
		t.Fatal("collapsing the sidebar immediately hid/disabled the bottom bar")
	}
	if err := runIPCCommand(endpoint, []any{"script-message-to", "playlist_sidebar", "toggle"}); err != nil {
		t.Fatal(err)
	}
	waitMPVTestProperty(t, endpoint, "user-data/osc/visibility", "always")
	if readMPVTestProperty(t, endpoint, "osd-width") != width || readMPVTestProperty(t, endpoint, "osd-height") != height {
		t.Fatal("sidebar toggle changed the window size")
	}
}
