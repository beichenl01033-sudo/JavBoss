//go:build windows

package mpv

import (
	"math"
	"runtime"
	"unsafe"
)

var (
	dockGetWindowRect     = moduser32.NewProc("GetWindowRect")
	dockGetClientRect     = moduser32.NewProc("GetClientRect")
	dockMonitorFromWindow = moduser32.NewProc("MonitorFromWindow")
	dockGetMonitorInfo    = moduser32.NewProc("GetMonitorInfoW")
	dockSetWindowPos      = moduser32.NewProc("SetWindowPos")
	dockIsZoomed          = moduser32.NewProc("IsZoomed")
	dockSetDPI            = moduser32.NewProc("SetThreadDpiAwarenessContext")
)

type dockRect struct{ Left, Top, Right, Bottom int32 }
type dockMonitorInfo struct {
	Size          uint32
	Monitor, Work dockRect
	Flags         uint32
}

func dockDPIContext() func() {
	runtime.LockOSThread()
	var old uintptr
	if dockSetDPI.Find() == nil {
		old, _, _ = dockSetDPI.Call(^uintptr(3))
	}
	return func() {
		if old != 0 {
			dockSetDPI.Call(old)
		}
		runtime.UnlockOSThread()
	}
}

func readDockWindow(pid int) (dockWindowInfo, bool) {
	defer dockDPIContext()()
	hwnd := findProcessWindow(uint32(pid))
	if hwnd == 0 {
		return dockWindowInfo{}, false
	}
	var frame, client dockRect
	if ok, _, _ := dockGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&frame))); ok == 0 {
		return dockWindowInfo{}, false
	}
	if ok, _, _ := dockGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&client))); ok == 0 {
		return dockWindowInfo{}, false
	}
	monitor, _, _ := dockMonitorFromWindow.Call(uintptr(hwnd), 2)
	info := dockMonitorInfo{Size: uint32(unsafe.Sizeof(dockMonitorInfo{}))}
	if ok, _, _ := dockGetMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return dockWindowInfo{}, false
	}
	maximized, _, _ := dockIsZoomed.Call(uintptr(hwnd))
	return dockWindowInfo{
		X: float64(frame.Left), Y: float64(frame.Top), Width: float64(frame.Right - frame.Left), Height: float64(frame.Bottom - frame.Top),
		ClientWidth: float64(client.Right - client.Left), ClientHeight: float64(client.Bottom - client.Top), WorkX: float64(info.Work.Left), WorkY: float64(info.Work.Top),
		WorkWidth: float64(info.Work.Right - info.Work.Left), WorkHeight: float64(info.Work.Bottom - info.Work.Top), PixelScale: 1, Maximized: maximized != 0,
	}, true
}

func resizeDockWindow(_ string, pid int, _ windowDockRequest, info dockWindowInfo, delta float64) (bool, string) {
	defer dockDPIContext()()
	hwnd := findProcessWindow(uint32(pid))
	if hwnd == 0 {
		return false, ""
	}
	// Preserve position, Z-order and focus; resize on the player's UI thread.
	ok, _, _ := dockSetWindowPos.Call(uintptr(hwnd), 0, 0, 0, uintptr(math.Round(info.Width+delta)), uintptr(math.Round(info.Height)), 0x0002|0x0004|0x0010|0x4000)
	return ok != 0, ""
}

func releaseDockGeometry(string, string, windowDockRequest, dockWindowInfo) bool { return true }
