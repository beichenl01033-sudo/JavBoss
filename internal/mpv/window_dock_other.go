//go:build !darwin && !windows

package mpv

func readDockWindow(int) (dockWindowInfo, bool) { return dockWindowInfo{}, false }
func resizeDockWindow(string, int, windowDockRequest, dockWindowInfo, float64) (bool, string) {
	return false, ""
}
func releaseDockGeometry(string, string, windowDockRequest, dockWindowInfo) bool { return true }
