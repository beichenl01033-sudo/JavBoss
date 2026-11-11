package mpv

import (
	"context"
	"encoding/json"
	"math"
	"testing"
)

func TestWindowDockPlansPreserveControlsAndOwnedWidth(t *testing.T) {
	for _, tc := range []struct {
		name                                    string
		width, preferred, extra, x, work, scale float64
		fullscreen, maximized                   bool
		wantDelta, wantControls, wantExtra      float64
	}{
		{"expand", 960, 560, 0, 100, 1920, 1, false, false, 560, 960, 560},
		{"no accumulation", 1520, 560, 560, 100, 1920, 1, false, false, 0, 960, 560},
		{"close preview", 1520, 320, 560, 100, 1920, 1, false, false, -240, 960, 320},
		{"close sidebar", 1520, 0, 560, 100, 1920, 1, false, false, -560, 960, 0},
		{"not enough right space", 960, 560, 0, 700, 1920, 1, false, false, 0, 960, 0},
		{"manual resize", 1620, 560, 560, 100, 1920, 1, false, false, 0, 1060, 560},
		{"manual resize beyond left edge", 1060, 320, 320, -30, 1920, 1, false, false, 0, 740, 320},
		{"move to smaller screen", 1520, 560, 560, 100, 1280, 1, false, false, -560, 960, 0},
		{"fullscreen", 1920, 0, 560, 0, 1920, 1, true, false, 0, 1920, 560},
		{"maximized", 1920, 560, 560, 0, 1920, 1, false, true, 0, 1920, 560},
		{"retina expand", 1920, 1120, 0, 100, 1920, 2, false, false, 560, 1920, 1120},
		{"retina restore", 3040, 0, 560, 100, 1920, 2, false, false, -560, 1920, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := windowDockRequest{Width: tc.width, Height: 640, Preferred: tc.preferred, Fullscreen: tc.fullscreen, Maximized: tc.maximized}
			info := dockWindowInfo{X: tc.x, Width: tc.width / tc.scale, ClientWidth: tc.width / tc.scale, WorkWidth: tc.work, PixelScale: tc.scale}
			plan := planWindowDock(request, info, tc.extra)
			if math.Abs(plan.Delta-tc.wantDelta) > 0.01 || math.Abs(plan.Layout.ControlsWidth-tc.wantControls) > 0.01 {
				t.Fatalf("unexpected dock plan: %+v", plan)
			}
			if !plan.Suspended && math.Abs(plan.Layout.ExtraWidth-tc.wantExtra) > 0.01 {
				t.Fatalf("unexpected extra width: %+v", plan)
			}
			if !plan.Suspended && math.Abs(plan.Layout.WindowWidth-(tc.width+tc.wantDelta*tc.scale)) > 0.01 {
				t.Fatalf("unexpected target size: %+v", plan)
			}
		})
	}
}

func TestWindowDockRejectsInvalidAndStaleRequests(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	dock := &windowDock{ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1)}
	defer cancel()
	for _, raw := range []string{`null`, `{}`, `{"serial":1,"width":960,"height":640,"preferred":9000}`, `{"serial":1,"width":-2,"height":640}`} {
		dock.request(json.RawMessage(raw))
		if dock.latest.Serial != 0 {
			t.Fatal("accepted invalid request")
		}
	}
	dock.request(json.RawMessage(`{"serial":2,"width":960,"height":640,"preferred":560}`))
	dock.request(json.RawMessage(`{"serial":1,"width":960,"height":640,"preferred":0}`))
	if dock.latest.Serial != 2 || dock.latest.Preferred != 560 {
		t.Fatal("stale request replaced newest layout")
	}
	cancel()
	dock.request(json.RawMessage(`{"serial":3,"width":960,"height":640,"preferred":0}`))
	if dock.latest.Serial != 2 {
		t.Fatal("request accepted after player shutdown")
	}
}

func TestWindowDockSuspendsNativeMinimizedWindow(t *testing.T) {
	info := dockWindowInfo{X: -32000, Y: -32000, Width: 160, Height: 40, ClientWidth: 960, PixelScale: 1, Minimized: true}
	plan := planWindowDock(windowDockRequest{Width: 960, Height: 640, Preferred: 560}, info, 320)
	if !plan.Suspended || plan.Delta != 0 || plan.Extra != 320 {
		t.Fatalf("minimize lost ownership or resized: %+v", plan)
	}
}

func TestWindowDockSuspendsFullscreenRestoreTransition(t *testing.T) {
	info := dockWindowInfo{Width: 1920, Height: 1080, ClientWidth: 1920, ClientHeight: 1080, WorkWidth: 1920, WorkHeight: 1040, PixelScale: 1}
	plan := planWindowDock(windowDockRequest{Width: 1920, Height: 1080, Preferred: 0}, info, 320)
	if !plan.Suspended || plan.Delta != 0 || plan.Extra != 320 {
		t.Fatalf("fullscreen restore lost the normal window baseline: %+v", plan)
	}
}

func TestWindowDockReconcilesStaleCocoaMaximizedFlag(t *testing.T) {
	info := dockWindowInfo{Width: 960, Height: 480, ClientWidth: 960, ClientHeight: 480, WorkWidth: 1920, WorkHeight: 960, PixelScale: 1, NativeState: true, GeometryState: true}
	request := windowDockRequest{Width: 960, Height: 480, Preferred: 320, Maximized: true}
	plan := planWindowDock(request, info, 320)
	if plan.Suspended || plan.Layout.ControlsWidth != 640 {
		t.Fatalf("stale maximize flag prevented normal restore: %+v", plan)
	}
	info.Height = 960
	plan = planWindowDock(request, info, 320)
	if !plan.Suspended {
		t.Fatal("actual Cocoa zoom state was ignored")
	}
}
