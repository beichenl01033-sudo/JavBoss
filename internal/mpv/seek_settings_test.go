package mpv

import (
	"context"
	"strings"
	"testing"
)

func TestSeekAndDecoderConfiguration(t *testing.T) {
	store := openConfigTestDB(t)
	ctx := context.Background()
	for _, tc := range []struct{ mode, hwdec, binding string }{
		{"adaptive", "no", "RIGHT script-binding javboss_control/adaptive-seek 2.5"},
		{"exact", "auto", "RIGHT no-osd seek 2.5 exact"},
	} {
		if err := store.UpsertConfig(ctx, map[string]string{
			playerHotkeysConfigKey:  `[{"key":"ArrowRight","action":"seek","amount":2.5},{"key":"ArrowLeft","action":"seek","amount":-2.5}]`,
			playerSeekModeConfigKey: tc.mode, playerHWDecConfigKey: tc.hwdec,
		}); err != nil {
			t.Fatal(err)
		}
		input, err := buildInputConfContent()
		if err != nil || !strings.Contains(input, tc.binding+"\n") {
			t.Fatalf("input %q: %v", input, err)
		}
		config, err := buildConfigContent()
		if err != nil || !strings.Contains(config, "hwdec="+tc.hwdec+"\n") {
			t.Fatalf("config %q: %v", config, err)
		}
	}
}
