//go:build !windows

package mpv

func shouldStageClipLocally(string) bool {
	return false
}
