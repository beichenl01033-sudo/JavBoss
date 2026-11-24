package ltx

import "testing"

func TestMarked(t *testing.T) {
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"ABC-123_LTX.mp4", true}, {"ABC-123.restored_lTx.mkv", true},
		{"[LTX] ABC-123_CD1.mp4", true}, {"ABC-123.mp4", false},
		{"/LTX/ABC-123.mp4", false}, {`Z:\LTX\ABC-123.mp4`, false},
		{"ABC-123_LTX2.mp4", false}, {"ABC-123_NOTLTX.mp4", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Marked(tc.name); got != tc.want {
				t.Fatalf("Marked(%q) = %v", tc.name, got)
			}
		})
	}
}
