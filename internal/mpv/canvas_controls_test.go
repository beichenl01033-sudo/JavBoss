package mpv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCanvasAndControlVisibilityLua(t *testing.T) {
	path := os.Getenv("JAVBOSS_TEST_MPV")
	if path == "" {
		t.Skip("set JAVBOSS_TEST_MPV to run mouse and sidebar control visibility tests")
	}
	sourceDir, err := findModernZSourceDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, harness, source, marker string }{
		{"click", "canvas_controls.lua", "javboss_control.lua", "CANVAS_CLICK_TESTS_PASSED"},
		{"controls", "control_visibility.lua", "modernz.lua", "CONTROL_VISIBILITY_TESTS_PASSED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, path, "--no-config", "--load-scripts=no", "--idle=yes", "--vo=null", "--ao=null",
				"--script="+filepath.Join("testdata", tc.harness), "--script-opts=test-script="+filepath.Join(sourceDir, tc.source))
			output, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(output), tc.marker) {
				t.Fatalf("Lua regression: %v\n%s", err, output)
			}
		})
	}
}
