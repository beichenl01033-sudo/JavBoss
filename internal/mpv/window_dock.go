package mpv

import (
	"context"
	"encoding/json"
	"math"
	"sync"
	"time"
)

const windowDockRequestProperty = "user-data/javboss/window-dock-request"
const windowDockLayoutProperty = "user-data/javboss/window-dock-layout"

type windowDockRequest struct {
	Serial     int64   `json:"serial"`
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
	Preferred  float64 `json:"preferred"`
	Fullscreen bool    `json:"fullscreen"`
	Maximized  bool    `json:"maximized"`
	Hidpi      bool    `json:"hidpi"`
}

type dockWindowInfo struct {
	X, Y, Width, Height float64
	ClientWidth         float64
	ClientHeight        float64
	WorkX, WorkY        float64
	WorkWidth           float64
	WorkHeight          float64
	PixelScale          float64
	Maximized           bool
}

type windowDockLayout struct {
	Serial        int64   `json:"serial"`
	ControlsWidth float64 `json:"controls_width"`
	WindowWidth   float64 `json:"window_width"`
	ExtraWidth    float64 `json:"extra_width"`
	Expanded      bool    `json:"expanded"`
}

type dockPlan struct {
	Layout    windowDockLayout
	Extra     float64
	Delta     float64
	Suspended bool
}

// Extra is only the width added by us. User moves and manual resizes never
// restore an old rectangle or recenter the window.
func planWindowDock(request windowDockRequest, info dockWindowInfo, extra float64) dockPlan {
	plan := dockPlan{Extra: extra, Layout: windowDockLayout{Serial: request.Serial, ControlsWidth: request.Width, WindowWidth: request.Width}}
	if request.Fullscreen || request.Maximized || info.Maximized {
		plan.Suspended = true
		return plan
	}
	scale := info.PixelScale
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return plan
	}
	base := info.ClientWidth - extra
	if base < 160/scale {
		// A window manager may have clamped a restored window to a new screen.
		extra, plan.Extra, base = 0, 0, info.ClientWidth
	}
	wanted := request.Preferred / scale
	frameWidth := info.Width - extra + wanted
	if wanted <= 0 || info.X < info.WorkX-2 || info.X+frameWidth > info.WorkX+info.WorkWidth-2 {
		wanted = 0
	}
	plan.Extra = wanted
	plan.Delta = wanted - extra
	plan.Layout = windowDockLayout{
		Serial:        request.Serial,
		ControlsWidth: base * scale,
		WindowWidth:   request.Width + plan.Delta*scale,
		ExtraWidth:    wanted * scale,
		Expanded:      wanted > 0,
	}
	return plan
}

type windowDock struct {
	mu       sync.Mutex
	latest   windowDockRequest
	wake     chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
	endpoint string
}

func newWindowDock(endpoint string) *windowDock {
	ctx, cancel := context.WithCancel(context.Background())
	dock := &windowDock{endpoint: endpoint, wake: make(chan struct{}, 1), ctx: ctx, cancel: cancel}
	go dock.run()
	return dock
}

func (d *windowDock) request(raw json.RawMessage) {
	var request windowDockRequest
	if len(raw) > 2048 || json.Unmarshal(raw, &request) != nil || request.Serial <= 0 ||
		request.Width < 1 || request.Width > 32000 || request.Height < 1 || request.Height > 32000 ||
		request.Preferred < 0 || request.Preferred > 8000 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ctx.Err() != nil || request.Serial <= d.latest.Serial {
		return
	}
	d.latest = request
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

func (d *windowDock) current(serial int64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.ctx.Err() == nil && d.latest.Serial == serial
}

func (d *windowDock) run() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var pid int
	var extra float64
	var expectedWidth float64
	var resizeAt time.Time
	var published []byte
	var processed int64
	var pendingBase float64
	var pendingTarget float64
	var denied [7]float64
	var hasDenied bool
	var ownedGeometry string
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-d.wake:
		case <-ticker.C:
		}
		d.mu.Lock()
		request := d.latest
		d.mu.Unlock()
		if request.Serial == 0 || (request.Preferred == 0 && extra == 0 && expectedWidth == 0 && processed == request.Serial) {
			continue
		}
		if pid == 0 {
			data, err := queryIPCCommand(d.endpoint, []any{"get_property", "pid"})
			if err != nil || json.Unmarshal(data, &pid) != nil {
				continue
			}
		}
		layout := windowDockLayout{Serial: request.Serial, ControlsWidth: request.Width, WindowWidth: request.Width}
		if info, ok := readDockWindow(pid); ok {
			// OSD and native geometry notifications can arrive in either order.
			if math.Abs(info.ClientWidth*info.PixelScale-request.Width) > 3 || math.Abs(info.ClientHeight*info.PixelScale-request.Height) > 3 {
				continue
			}
			if expectedWidth != 0 && !request.Fullscreen && !request.Maximized && !info.Maximized && math.Abs(info.ClientWidth-pendingTarget)*info.PixelScale > 3 {
				if time.Since(resizeAt) < time.Second {
					continue
				}
				// The OS refused or clamped a resize. Track only the width that
				// actually appeared, then fall back instead of growing repeatedly.
				extra = math.Max(0, info.ClientWidth-pendingBase)
				denied = [7]float64{request.Preferred, info.X, info.Y, info.WorkX, info.WorkY, info.WorkWidth, info.ClientWidth - extra}
				hasDenied = true
			}
			expectedWidth = 0
			if ownedGeometry != "" && !request.Fullscreen && !request.Maximized && !info.Maximized {
				if releaseDockGeometry(d.endpoint, ownedGeometry, request, info) {
					ownedGeometry = ""
				}
			}
			key := [7]float64{request.Preferred, info.X, info.Y, info.WorkX, info.WorkY, info.WorkWidth, info.ClientWidth - extra}
			wanted := request
			if hasDenied && key == denied {
				wanted.Preferred = 0
			} else {
				hasDenied = false
			}
			plan := planWindowDock(wanted, info, extra)
			layout = plan.Layout
			if !plan.Suspended && math.Abs(plan.Delta) > 0.5 {
				if !d.current(request.Serial) {
					continue
				}
				// Publish the original control width before the window grows so the
				// OSC never recenters itself into the new sidebar area.
				_ = runIPCCommand(d.endpoint, []any{"set_property", windowDockLayoutProperty, layout})
				accepted, geometry := resizeDockWindow(d.endpoint, pid, request, info, plan.Delta)
				if accepted {
					ownedGeometry = geometry
					pendingBase = info.ClientWidth - extra
					pendingTarget = info.ClientWidth + plan.Delta
					extra = plan.Extra
					expectedWidth = request.Width + plan.Delta*info.PixelScale
					resizeAt = time.Now()
				} else {
					denied, hasDenied = key, true
					layout = windowDockLayout{Serial: request.Serial, ControlsWidth: (info.ClientWidth - extra) * info.PixelScale,
						WindowWidth: request.Width, ExtraWidth: extra * info.PixelScale, Expanded: extra > 0}
				}
			}
		}
		encoded, _ := json.Marshal(layout)
		if string(encoded) != string(published) && d.current(request.Serial) {
			_ = runIPCCommand(d.endpoint, []any{"set_property", windowDockLayoutProperty, layout})
			published = encoded
		}
		processed = request.Serial
	}
}
