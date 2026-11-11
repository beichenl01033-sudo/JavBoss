//go:build darwin

package mpv

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestAdaptiveWindowDockWithMPV(t *testing.T) {
	mpvPath := os.Getenv("JAVBOSS_TEST_MPV_GUI")
	if mpvPath == "" {
		t.Skip("set JAVBOSS_TEST_MPV_GUI for real native window docking")
	}
	if os.Getenv("JAVBOSS_MPV_DOCK_HELPER") == "" {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(self, "-test.run=^TestAdaptiveWindowDockWithMPV$", "-test.v")
		cmd.Env = append(os.Environ(), "JAVBOSS_MPV_DOCK_HELPER=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("dock integration: %v\n%s", err, output)
		}
		return
	}
	root := t.TempDir()
	wrapper := filepath.Join(root, "mpv-dock")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec \""+mpvPath+"\" \"$@\" --ao=null --native-fs=no --geometry=640x480+20+40 --script-opts=playlist_sidebar-cover_preview=no --image-display-duration=inf --title='JavBoss Adaptive Sidebar Test'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MPV_PATH", wrapper)
	source, _ := filepath.Abs("../../modernz")
	t.Setenv(modernZEnvDir, source)
	image, _ := filepath.Abs("../../web/public/icon-192.png")
	session := &playerSession{}
	t.Cleanup(session.Reset)
	if err := session.PlayPlaylistFrom([]PlaylistItem{{Path: image, Title: "Dock test", Options: PlayOptions{VideoID: 1}}}, 0); err != nil {
		t.Fatal(err)
	}
	endpoint := session.ipcPath
	command := func(args ...any) {
		t.Helper()
		if err := runIPCCommand(endpoint, args); err != nil {
			t.Fatal(err)
		}
	}
	command("set_property", "pause", true)
	command("show-text", "", 0)
	command("script-message-to", "modernz", "osc-visibility", "always", "yes")
	capture := func(suffix string) {
		t.Helper()
		if target := os.Getenv("JAVBOSS_DOCK_PREVIEW_PATH"); target != "" {
			command("show-text", "", 0)
			time.Sleep(100 * time.Millisecond)
			command("screenshot-to-file", target+suffix+".png", "window")
		}
	}
	waitMPVTestProperty(t, endpoint, "osd-width", float64(640))
	pid := session.cmd.Process.Pid
	before, ok := readDockWindow(pid)
	deadline := time.Now().Add(2 * time.Second)
	for !ok || math.Abs(before.ClientWidth*before.PixelScale-640) > 1 || math.Abs(before.ClientHeight*before.PixelScale-480) > 1 {
		if time.Now().After(deadline) {
			t.Fatalf("native opening animation did not settle: %+v", before)
		}
		time.Sleep(20 * time.Millisecond)
		before, ok = readDockWindow(pid)
	}
	if !ok {
		t.Fatal("native window bounds unavailable")
	}
	t.Logf("before: %+v, osd=%v x %v", before, readMPVTestProperty(t, endpoint, "osd-width"), readMPVTestProperty(t, endpoint, "osd-height"))
	capture(".closed")
	if before.X+before.Width+320 > before.WorkX+before.WorkWidth-2 {
		t.Skip("not enough monitor room for the expansion regression")
	}
	command("script-message-to", "playlist_sidebar", "show")
	waitMPVTestProperty(t, endpoint, "user-data/javboss/window-dock-layout/expanded", true)
	waitMPVTestProperty(t, endpoint, "osd-width", float64(960))
	waitMPVTestProperty(t, endpoint, "geometry", "960x480")
	waitMPVTestProperty(t, endpoint, "user-data/javboss/window-dock-layout/controls_width", float64(640))
	time.Sleep(150 * time.Millisecond)
	after, ok := readDockWindow(pid)
	if !ok {
		t.Fatal("expanded bounds unavailable")
	}
	t.Logf("after: %+v, osd=%v x %v", after, readMPVTestProperty(t, endpoint, "osd-width"), readMPVTestProperty(t, endpoint, "osd-height"))
	capture(".expanded")
	for _, pair := range [][2]float64{{after.X, before.X}, {after.Y, before.Y}, {after.Height, before.Height}, {after.Width, before.Width + 320}} {
		if math.Abs(pair[0]-pair[1]) > 2 {
			t.Fatalf("native bounds changed incorrectly: before=%+v after=%+v", before, after)
		}
	}
	// Fullscreen may shrink the video for a sidebar, but must never resize the
	// native fullscreen window or consume its saved non-fullscreen extra width.
	screenWidth := readMPVTestProperty(t, endpoint, "display-width")
	command("set_property", "fullscreen", true)
	waitMPVTestProperty(t, endpoint, "osd-width", screenWidth)
	waitMPVTestProperty(t, endpoint, "user-data/javboss/window-dock-layout/controls_width", screenWidth)
	command("script-message-to", "playlist_sidebar", "hide")
	waitMPVTestProperty(t, endpoint, "user-data/javboss/playlist-sidebar-width", float64(0))
	if readMPVTestProperty(t, endpoint, "osd-width") != screenWidth {
		t.Fatal("hiding resized fullscreen")
	}
	command("script-message-to", "playlist_sidebar", "show")
	waitMPVTestProperty(t, endpoint, "user-data/javboss/playlist-sidebar-width", float64(320))
	command("set_property", "fullscreen", false)
	waitMPVTestProperty(t, endpoint, "osd-width", float64(960))
	waitMPVTestProperty(t, endpoint, "user-data/javboss/window-dock-layout/controls_width", float64(640))
	command("script-message-to", "playlist_sidebar", "hide")
	waitMPVTestProperty(t, endpoint, "osd-width", float64(640))
	waitMPVTestProperty(t, endpoint, "video-margin-ratio-right", float64(0))
	waitMPVTestProperty(t, endpoint, "user-data/osc/visibility", "auto")
	command("script-message-to", "playlist_sidebar", "show")
	waitMPVTestProperty(t, endpoint, "osd-width", float64(960))
	command("script-message-to", "playlist_sidebar", "hide")
	waitMPVTestProperty(t, endpoint, "osd-width", float64(640))
	final, _ := readDockWindow(pid)
	if math.Abs(final.Width-before.Width) > 2 {
		t.Fatal("repeated toggles accumulated window width")
	}
	// A manual resize becomes the new baseline, rather than restoring an old
	// rectangle on collapse. Subsequent files must not reapply temporary anchors.
	command("script-message-to", "playlist_sidebar", "show")
	waitMPVTestProperty(t, endpoint, "geometry", "960x480")
	command("set_property", "geometry", "1060x480")
	waitMPVTestProperty(t, endpoint, "osd-width", float64(1060))
	waitMPVTestProperty(t, endpoint, "user-data/javboss/window-dock-layout/controls_width", float64(740))
	secondImage, _ := filepath.Abs("../../web/public/icon-512.png")
	command("loadfile", secondImage, "append")
	command("playlist-play-index", 1)
	waitMPVTestProperty(t, endpoint, "video-out-params/w", float64(512))
	waitMPVTestProperty(t, endpoint, "osd-width", float64(1060))
	command("script-message-to", "playlist_sidebar", "hide")
	waitMPVTestProperty(t, endpoint, "osd-width", float64(740))
	command("set_property", "geometry", "640x480+20+40")
	waitMPVTestProperty(t, endpoint, "osd-width", float64(640))

	// Near the monitor's right edge, keep the entire window and toolbar still
	// and fit the video into the space left beside the sidebar.
	rightX := before.WorkWidth - before.Width - 20
	command("set_property", "geometry", fmt.Sprintf("640x480+%d+40", int(rightX)))
	time.Sleep(150 * time.Millisecond)
	cramped, _ := readDockWindow(pid)
	command("script-message-to", "playlist_sidebar", "show")
	waitMPVTestProperty(t, endpoint, "video-margin-ratio-right", 0.5)
	request := readMPVTestProperty(t, endpoint, windowDockRequestProperty).(map[string]any)
	waitMPVTestProperty(t, endpoint, "user-data/javboss/window-dock-layout/serial", request["serial"])
	waitMPVTestProperty(t, endpoint, "user-data/javboss/window-dock-layout/expanded", false)
	waitMPVTestProperty(t, endpoint, "user-data/javboss/window-dock-layout/controls_width", float64(640))
	got, _ := readDockWindow(pid)
	capture(".fallback")
	if math.Abs(got.X-cramped.X) > 2 || math.Abs(got.Width-cramped.Width) > 2 || math.Abs(got.Height-cramped.Height) > 2 {
		t.Fatalf("fallback moved the player: before=%+v after=%+v", cramped, got)
	}
	command("script-message-to", "playlist_sidebar", "hide")
	waitMPVTestProperty(t, endpoint, "video-margin-ratio-right", float64(0))
	command("set_property", "geometry", "640x480+20+40")
	time.Sleep(150 * time.Millisecond)
	command("script-message-to", "playlist_sidebar", "show")
	waitMPVTestProperty(t, endpoint, "osd-width", float64(960))
	if before.X+before.Width+560 < before.WorkX+before.WorkWidth-2 {
		command("script-message-to", "playlist_sidebar", "toggle-cover-preview")
		waitMPVTestProperty(t, endpoint, "osd-width", float64(1200))
		waitMPVTestProperty(t, endpoint, "user-data/javboss/window-dock-layout/controls_width", float64(640))
		command("script-message-to", "playlist_sidebar", "toggle-cover-preview")
		waitMPVTestProperty(t, endpoint, "osd-width", float64(960))
	}
}
