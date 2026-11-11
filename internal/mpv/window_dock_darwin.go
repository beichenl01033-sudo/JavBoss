//go:build darwin

package mpv

/*
#include <stdbool.h>
bool javbossDockWindowInfo(int pid, double *values);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"math"
)

func readDockWindow(pid int) (dockWindowInfo, bool) {
	var values [9]C.double
	if !bool(C.javbossDockWindowInfo(C.int(pid), &values[0])) {
		return dockWindowInfo{}, false
	}
	return dockWindowInfo{
		X: float64(values[0]), Y: float64(values[1]), Width: float64(values[2]), Height: float64(values[3]),
		ClientWidth: float64(values[2]), ClientHeight: float64(values[3]), WorkX: float64(values[4]), WorkY: float64(values[5]),
		WorkWidth: float64(values[6]), WorkHeight: float64(values[7]), PixelScale: float64(values[8]), NativeState: true, GeometryState: true,
	}, true
}

func resizeDockWindow(endpoint string, _ int, request windowDockRequest, info dockWindowInfo, delta float64) (bool, string) {
	scale := info.PixelScale
	units := scale
	if request.Hidpi {
		units = 1
	}
	width := int(math.Round((info.ClientWidth + delta) * units))
	height := int(math.Round(request.Height / scale * units))
	x := int(math.Round((info.X - info.WorkX) * units))
	y := int(math.Round((info.Y - info.WorkY) * units))
	geometry := fmt.Sprintf("%dx%d+%d+%d", width, height, x, y)
	return runIPCCommand(endpoint, []any{"set_property", "geometry", geometry}) == nil, geometry
}

func releaseDockGeometry(endpoint, owned string, request windowDockRequest, info dockWindowInfo) bool {
	data, err := queryIPCCommand(endpoint, []any{"get_property", "geometry"})
	var current string
	if err != nil || json.Unmarshal(data, &current) != nil {
		return false
	}
	if current != owned {
		return true
	}
	units := info.PixelScale
	if request.Hidpi {
		units = 1
	}
	size := fmt.Sprintf("%dx%d", int(math.Round(request.Width/info.PixelScale*units)), int(math.Round(request.Height/info.PixelScale*units)))
	// Drop only our temporary coordinates after a resize. Matching the current
	// size avoids reapplying old coordinates on subsequent file changes.
	return runIPCCommand(endpoint, []any{"set_property", "geometry", size}) == nil
}
