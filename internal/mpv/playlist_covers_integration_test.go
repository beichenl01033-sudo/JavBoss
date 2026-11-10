//go:build !windows

package mpv

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

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
		flags = " --ao=null --force-window=yes --geometry=960x640 --image-display-duration=inf --title='JavBoss Cover Playlist Preview'"
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
	data := coverFixture(t)
	var firstLoads, secondLoads atomic.Int64
	items := []PlaylistItem{
		{Path: media, Title: "Video A", Options: PlayOptions{VideoID: 1}, Cover: func(context.Context) (PlaylistCoverSource, error) {
			firstLoads.Add(1)
			return PlaylistCoverSource{Data: data}, nil
		}},
		{Path: media, Title: "Video B", Options: PlayOptions{VideoID: 2}, Cover: func(context.Context) (PlaylistCoverSource, error) {
			secondLoads.Add(1)
			return PlaylistCoverSource{Data: data}, nil
		}},
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
		waitMPVTestProperty(t, endpoint, "user-data/javboss/playlist-cover-count", float64(2))
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
		waitMPVTestProperty(t, endpoint, "user-data/javboss/playlist-cover-count", float64(2))
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
