//go:build !windows

package mpv

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestHighlightNavigationAndPreviewWithMPV(t *testing.T) {
	if os.Getenv("JAVBOSS_TEST_MPV") == "" {
		t.Skip("set JAVBOSS_TEST_MPV to test actual Lua highlight playback")
	}
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "mpv-headless")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec \"$JAVBOSS_TEST_MPV\" \"$@\" --vo=null --ao=null --force-window=no --terminal=no --save-position-on-quit=no\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MPV_PATH", wrapper)
	source, err := filepath.Abs("../../modernz")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(modernZEnvDir, source)
	wave := make([]byte, 44+8000*2*12)
	copy(wave, "RIFF")
	binary.LittleEndian.PutUint32(wave[4:], uint32(len(wave)-8))
	copy(wave[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wave[16:], 16)
	binary.LittleEndian.PutUint16(wave[20:], 1)
	binary.LittleEndian.PutUint16(wave[22:], 1)
	binary.LittleEndian.PutUint32(wave[24:], 8000)
	binary.LittleEndian.PutUint32(wave[28:], 16000)
	binary.LittleEndian.PutUint16(wave[32:], 2)
	binary.LittleEndian.PutUint16(wave[34:], 16)
	copy(wave[36:], "data")
	binary.LittleEndian.PutUint32(wave[40:], uint32(len(wave)-44))
	items := []PlaylistItem{}
	for index := 0; index < 2; index++ {
		path := filepath.Join(dir, strconv.Itoa(index)+".wav")
		if err := os.WriteFile(path, wave, 0600); err != nil {
			t.Fatal(err)
		}
		id := int64(910 + index)
		videoDir := filepath.Join(dir, "video", strconv.FormatInt(id, 10))
		if err := os.MkdirAll(videoDir, 0755); err != nil {
			t.Fatal(err)
		}
		raw := `{"version":1,"items":[{"time":0,"label":"zero"},{"time":2,"label":"second"}]}`
		if index == 1 {
			raw = `{"version":1,"items":[{"time":4,"label":"next video"}]}`
		}
		if err := os.WriteFile(filepath.Join(videoDir, "highlights.json"), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		start := 0.0
		if index == 1 {
			start = 4
		}
		items = append(items, PlaylistItem{Path: path, Options: PlayOptions{DataDir: dir, VideoID: id, StartTimeSec: start, HighlightMode: true}})
	}
	session := &playerSession{}
	t.Cleanup(session.Reset)
	if err := session.PlayPlaylistFrom(items, 0); err != nil {
		t.Fatal(err)
	}
	endpoint := session.ipcPath
	waitMPVTestProperty(t, endpoint, "user-data/javboss/highlight-count", float64(2))
	command := func(args ...any) {
		t.Helper()
		if err := runIPCCommand(endpoint, args); err != nil {
			t.Fatal(err)
		}
	}
	position := func(min, max float64) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if at, ok := readMPVTestProperty(t, endpoint, "time-pos").(float64); ok && at >= min && at < max {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("unexpected time-pos=%v want [%v,%v)", readMPVTestProperty(t, endpoint, "time-pos"), min, max)
	}
	command("set_property", "pause", true)
	position(0, 1)
	command("script-message-to", "javboss_control", "next-highlight")
	position(1.7, 3)
	command("script-message-to", "javboss_control", "next-highlight")
	waitMPVTestProperty(t, endpoint, "playlist-pos", float64(1))
	position(3.7, 5)
	command("script-message-to", "javboss_control", "previous-highlight")
	waitMPVTestProperty(t, endpoint, "playlist-pos", float64(0))
	position(1.7, 3)
	// The same queue automatically previews all marks and stops on the final mark.
	for i := range items {
		items[i].Options.HighlightPreviewSeconds = 0.4
	}
	if err := session.PlayPlaylistFrom(items, 0); err != nil {
		t.Fatal(err)
	}
	waitMPVTestProperty(t, endpoint, "playlist-pos", float64(1))
	waitMPVTestProperty(t, endpoint, "pause", true)
	position(3.7, 5.5)
	// A Web edit appears in the open player without loading another video.
	path := filepath.Join(dir, "video", "911", "highlights.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"items":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	waitMPVTestProperty(t, endpoint, "user-data/javboss/highlight-count", float64(0))
}
