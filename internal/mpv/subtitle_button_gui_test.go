//go:build !windows

package mpv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSubtitleButtonWithNativeMPV(t *testing.T) {
	executable := os.Getenv("JAVBOSS_TEST_MPV_GUI")
	if executable == "" {
		t.Skip("set JAVBOSS_TEST_MPV_GUI for the real subtitle button")
	}
	if os.Getenv("JAVBOSS_SUBTITLE_BUTTON_CHILD") == "" {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(self, "-test.run=^TestSubtitleButtonWithNativeMPV$", "-test.v")
		cmd.Env = append(os.Environ(), "JAVBOSS_SUBTITLE_BUTTON_CHILD=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("native subtitle button: %v\n%s", err, output)
		}
		return
	}
	root := t.TempDir()
	media, subtitles := filepath.Join(root, "preview.mp4"), filepath.Join(root, "test.srt")
	ffmpeg := os.Getenv("JAVBOSS_TEST_FFMPEG")
	if ffmpeg == "" {
		t.Skip("set JAVBOSS_TEST_FFMPEG to generate a synthetic test video")
	}
	if output, err := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=0x20344b:s=960x540:r=5", "-t", "5", "-c:v", "mpeg4", media).CombinedOutput(); err != nil {
		t.Fatalf("create synthetic video: %v\n%s", err, output)
	}
	if err := os.WriteFile(subtitles, []byte("1\n00:00:00,000 --> 01:00:00,000\nSubtitle preview\n"), 0600); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(root, "mpv-subtitle-gui")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec \""+executable+"\" \"$@\" --ao=null --force-window=yes --pause=yes --native-fs=no --image-display-duration=inf --geometry=960x640+20+40 --title='JavBoss Subtitle Status Test'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MPV_PATH", wrapper)
	source, err := filepath.Abs("../../modernz")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(modernZEnvDir, source)
	session := &playerSession{}
	t.Cleanup(session.Reset)
	if err := session.PlayPlaylist([]PlaylistItem{{Path: media, Title: "Subtitle status preview", Options: PlayOptions{
		ResolveSubtitle: func(_ context.Context, request SubtitleRequest) (*SubtitleFile, error) {
			if request.SettingsOnly {
				return &SubtitleFile{}, nil
			}
			return &SubtitleFile{Path: subtitles, Language: "zh-CN", Key: "download:zh-CN", Status: "ready", Downloaded: true}, nil
		},
		SaveSubtitleOffset: func(context.Context, int64) error { return nil },
	}}}); err != nil {
		t.Fatal(err)
	}
	endpoint := session.ipcPath
	waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-status", "ready")
	waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-menu-items/0/selected", true)
	command := func(args ...any) {
		t.Helper()
		if err := runIPCCommand(endpoint, args); err != nil {
			t.Fatal(err)
		}
	}
	command("script-message-to", "modernz", "osc-visibility", "always", "yes")
	command("show-text", "", 0)
	for _, stage := range []struct{ name, status string }{{"enabled", "ready"}, {"searching", "searching"}, {"closed", "ready"}} {
		command("set_property", "user-data/javboss/subtitle-status", stage.status)
		command("set_property", "sub-visibility", stage.name != "closed")
		waitMPVTestProperty(t, endpoint, "user-data/javboss/subtitle-menu-items/0/selected", stage.name != "closed")
		if prefix := os.Getenv("JAVBOSS_SUBTITLE_PREVIEW_PATH"); prefix != "" {
			time.Sleep(200 * time.Millisecond)
			command("screenshot-to-file", prefix+"."+stage.name+".png", "window")
		}
	}
}
