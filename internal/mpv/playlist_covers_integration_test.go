//go:build !windows

package mpv

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// Synthetic posters make layout screenshots reviewable without user media.
func previewPosterFixture(t *testing.T, alternate bool) []byte {
	t.Helper()
	poster := image.NewRGBA(image.Rect(0, 0, 320, 480))
	for y := 0; y < 480; y++ {
		for x := 0; x < 320; x++ {
			c := color.RGBA{R: uint8(25 + y/12), G: uint8(65 + y/8), B: uint8(105 + x/5), A: 255}
			if alternate {
				c.R, c.B = c.B, c.R
			}
			poster.SetRGBA(x, y, c)
		}
	}
	glyph := []string{"01110", "10001", "10001", "11111", "10001", "10001", "10001"}
	if alternate {
		glyph = []string{"11110", "10001", "10001", "11110", "10001", "10001", "11110"}
	}
	for row, line := range glyph {
		for column, bit := range line {
			if bit == '1' {
				for y := 100 + row*34; y < 100+(row+1)*34; y++ {
					for x := 75 + column*34; x < 75+(column+1)*34; x++ {
						poster.SetRGBA(x, y, color.RGBA{R: 225, G: 235, B: 245, A: 255})
					}
				}
			}
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, poster); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestPlaylistCoversWithMPV(t *testing.T) {
	executable := os.Getenv("JAVBOSS_TEST_MPV")
	gui := os.Getenv("JAVBOSS_TEST_MPV_GUI")
	if gui != "" {
		executable = gui
	}
	if executable == "" {
		t.Skip("set JAVBOSS_TEST_MPV or JAVBOSS_TEST_MPV_GUI for cover integration")
	}
	if os.Getenv("JAVBOSS_MPV_COVER_HELPER") == "" {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(self, "-test.run=^TestPlaylistCoversWithMPV$", "-test.v")
		cmd.Env = append(os.Environ(), "JAVBOSS_MPV_COVER_HELPER=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cover integration: %v\n%s", err, output)
		}
		return
	}
	root := t.TempDir()
	wrapper := filepath.Join(root, "mpv-covers")
	flags := " --vo=null --ao=null --force-window=no --image-display-duration=inf"
	if gui != "" {
		flags = " --ao=null --force-window=yes --geometry=960x640 --script-opts=playlist_sidebar-dock_sidebar=no --image-display-duration=inf --title='JavBoss Cover Playlist Preview'"
	}
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec \""+executable+"\" \"$@\""+flags+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MPV_PATH", wrapper)
	source, err := filepath.Abs("../../modernz")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(modernZEnvDir, source)
	media := filepath.Join(root, "background.png")
	background := image.NewRGBA(image.Rect(0, 0, 960, 640))
	for y := 0; y < 640; y++ {
		for x := 0; x < 960; x++ {
			background.SetRGBA(x, y, color.RGBA{R: 32, G: 44, B: 56, A: 255})
		}
	}
	file, err := os.Create(media)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, background); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	firstData, secondData := previewPosterFixture(t, false), previewPosterFixture(t, true)
	var firstLoads, secondLoads atomic.Int64
	items := make([]PlaylistItem, 12)
	for index := range items {
		id := int64(index + 1)
		items[index] = PlaylistItem{Path: media, Title: fmt.Sprintf("Demo video %02d", id), Options: PlayOptions{VideoID: id}, Cover: func(context.Context) (PlaylistCoverSource, error) {
			data := firstData
			if id%2 == 0 {
				data = secondData
			}
			if id == 1 {
				firstLoads.Add(1)
			} else if id == 2 {
				secondLoads.Add(1)
			}
			return PlaylistCoverSource{Data: data}, nil
		}}
	}
	session := &playerSession{}
	t.Cleanup(session.Reset)
	if err := session.PlayPlaylistFrom(items, 0); err != nil {
		t.Fatal(err)
	}
	endpoint := session.ipcPath
	if gui != "" {
		_ = runIPCCommand(endpoint, []any{"show-text", "", 0})
	}
	if gui == "" {
		// With vo=null there is no viewport. Explicitly simulate a visible-cover
		// request through the actual IPC property observed by the Go event client.
		visuals := readMPVTestProperty(t, endpoint, "user-data/javboss/playlist-visuals").(map[string]any)
		entries := visuals["entries"].(map[string]any)
		keys := []any{}
		for _, entry := range entries {
			keys = append(keys, entry.(map[string]any)["key"])
		}
		if firstLoads.Load() != 0 || secondLoads.Load() != 0 {
			t.Fatal("hidden sidebar loaded artwork")
		}
		if err := runIPCCommand(endpoint, []any{"set_property", playlistCoverRequestProperty, map[string]any{"generation": visuals["generation"], "serial": 1, "keys": keys}}); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for firstLoads.Load() != 1 || secondLoads.Load() != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("cover requests not delivered: %d %d", firstLoads.Load(), secondLoads.Load())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if gui != "" {
		waitMPVTestProperty(t, endpoint, "playlist-count", float64(len(items)))
		dpi := readMPVTestProperty(t, endpoint, "display-hidpi-scale").(float64)
		logicalHeight := readMPVTestProperty(t, endpoint, "osd-height").(float64) / dpi
		visibleCovers := math.Floor((logicalHeight - 96 - 34 - 58) / 64)
		if visibleCovers < 6 || visibleCovers >= float64(len(items)) {
			t.Fatalf("unexpected visible list density: %v", visibleCovers)
		}
		waitMPVTestProperty(t, endpoint, "user-data/javboss/playlist-cover-count", visibleCovers+1)
		waitMPVTestProperty(t, endpoint, "user-data/javboss/playlist-preview-index", float64(0))
		originalWidth := readMPVTestProperty(t, endpoint, "osd-width")
		originalHeight := readMPVTestProperty(t, endpoint, "osd-height")
		if err := runIPCCommand(endpoint, []any{"script-message-to", "playlist_sidebar", "toggle-cover-preview"}); err != nil {
			t.Fatal(err)
		}
		waitMPVTestProperty(t, endpoint, "user-data/javboss/playlist-cover-count", visibleCovers)
		waitMPVTestProperty(t, endpoint, "user-data/javboss/playlist-preview-visible", false)
		waitMPVTestProperty(t, endpoint, "user-data/osc/visibility", "always")
		if err := runIPCCommand(endpoint, []any{"script-message-to", "playlist_sidebar", "toggle-cover-preview"}); err != nil {
			t.Fatal(err)
		}
		waitMPVTestProperty(t, endpoint, "user-data/javboss/playlist-cover-count", visibleCovers+1)
		waitMPVTestProperty(t, endpoint, "user-data/osc/visibility", "always")
		if readMPVTestProperty(t, endpoint, "osd-width") != originalWidth || readMPVTestProperty(t, endpoint, "osd-height") != originalHeight {
			t.Fatal("cover preview resized the player")
		}
		if readMPVTestProperty(t, endpoint, "video-margin-ratio-right") != float64(0) {
			t.Fatal("cover preview resized the video")
		}
		// Use MPV's own mouse command so actual Lua input routing is covered,
		// without moving the user's system pointer or clicking their windows.
		mouseX := originalWidth.(float64) - (240+100)*dpi
		mouseY := (58 + 64 + 20) * dpi
		if err := runIPCCommand(endpoint, []any{"mouse", mouseX, mouseY}); err != nil {
			t.Fatal(err)
		}
		waitMPVTestProperty(t, endpoint, "user-data/javboss/playlist-preview-index", float64(1))
		waitMPVTestProperty(t, endpoint, "playlist-pos", float64(0))
		if screenshot := os.Getenv("JAVBOSS_COVER_PREVIEW_PATH"); screenshot != "" {
			_ = runIPCCommand(endpoint, []any{"show-text", "", 0})
			time.Sleep(200 * time.Millisecond)
			if err := runIPCCommand(endpoint, []any{"screenshot-to-file", screenshot, "window"}); err != nil {
				t.Fatal(err)
			}
		}
		if err := runIPCCommand(endpoint, []any{"script-message-to", "playlist_sidebar", "hide"}); err != nil {
			t.Fatal(err)
		}
		waitMPVTestProperty(t, endpoint, "user-data/javboss/playlist-cover-count", float64(0))
	}
	points := []PlaylistItem{items[0], items[0], items[1]}
	for index := range points {
		id := int64(1)
		if index == 2 {
			id = 2
		}
		points[index].Highlight = &PlaylistHighlight{VideoID: id, VideoTitle: items[id-1].Title, Number: index + 1, Time: float64(index), Label: "Highlight"}
		points[index].Options.HighlightMode = true
	}
	if err := session.PlayPlaylistFrom(points, 1); err != nil {
		t.Fatal(err)
	}
	waitMPVTestProperty(t, endpoint, "playlist-count", float64(3))
	waitMPVTestProperty(t, endpoint, "playlist-pos", float64(1))
	if gui != "" {
		waitMPVTestProperty(t, endpoint, "user-data/javboss/playlist-cover-count", float64(3))
		waitMPVTestProperty(t, endpoint, "user-data/javboss/playlist-preview-index", float64(1))
		if firstLoads.Load() != 2 || secondLoads.Load() != 2 {
			t.Fatal("highlight entries requested duplicate covers")
		}
		if screenshot := os.Getenv("JAVBOSS_COVER_PREVIEW_PATH"); screenshot != "" {
			_ = runIPCCommand(endpoint, []any{"show-text", "", 0})
			time.Sleep(200 * time.Millisecond)
			if err := runIPCCommand(endpoint, []any{"screenshot-to-file", screenshot + ".highlights.png", "window"}); err != nil {
				t.Fatal(err)
			}
		}
	}
}
