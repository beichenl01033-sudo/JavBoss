//go:build windows

package mpv

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func shouldStageClipLocally(path string) bool {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if strings.HasPrefix(cleaned, `\\`) {
		return true
	}
	volume := filepath.VolumeName(cleaned)
	if volume == "" {
		return false
	}
	root, err := windows.UTF16PtrFromString(volume + `\`)
	if err != nil {
		return false
	}
	return windows.GetDriveType(root) == windows.DRIVE_REMOTE
}
