//go:build windows

package mpv

import (
	"math"
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestNativeWindowDockBoundsAndResize(t *testing.T) {
	defer dockDPIContext()()
	class, _ := windows.UTF16PtrFromString("STATIC")
	title, _ := windows.UTF16PtrFromString("JavBoss native docking test")
	create := moduser32.NewProc("CreateWindowExW")
	destroy := moduser32.NewProc("DestroyWindow")
	// The system STATIC class supplies its own window procedure. No desktop
	// interaction, player download, user data or administrator rights are needed.
	hwnd, _, err := create.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), 0x10CF0000, 20, 20, 400, 300, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatalf("create test window: %v", err)
	}
	defer destroy.Call(hwnd)
	before, ok := readDockWindow(os.Getpid())
	if !ok || before.ClientWidth <= 0 || before.ClientHeight <= 0 || before.WorkWidth <= 0 || before.PixelScale != 1 {
		t.Fatalf("invalid window info: %+v", before)
	}
	if accepted, _ := resizeDockWindow("", os.Getpid(), windowDockRequest{}, before, 120); !accepted {
		t.Fatal("native expansion failed")
	}
	after, ok := readDockWindow(os.Getpid())
	if !ok || math.Abs(after.Width-before.Width-120) > 1 || after.X != before.X || after.Y != before.Y || after.Height != before.Height {
		t.Fatalf("native expansion moved the player: before=%+v after=%+v", before, after)
	}
	if accepted, _ := resizeDockWindow("", os.Getpid(), windowDockRequest{}, after, -120); !accepted {
		t.Fatal("native restoration failed")
	}
	final, _ := readDockWindow(os.Getpid())
	if math.Abs(final.Width-before.Width) > 1 || final.X != before.X || final.Y != before.Y {
		t.Fatalf("native restoration drifted: before=%+v after=%+v", before, final)
	}
	procShowWindow.Call(hwnd, 6)
	minimized, ok := readDockWindow(os.Getpid())
	if !ok || !minimized.Minimized {
		t.Fatal("minimized window was not detected")
	}
	plan := planWindowDock(windowDockRequest{Width: before.ClientWidth, Height: before.ClientHeight, Preferred: 120}, minimized, 120)
	if !plan.Suspended || plan.Delta != 0 {
		t.Fatal("minimized window would be resized")
	}
	procShowWindow.Call(hwnd, 9)
}
