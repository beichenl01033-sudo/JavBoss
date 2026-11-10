package mpv

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func coverFixture(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 120, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 120; x++ {
			c := color.RGBA{R: 255, A: 255}
			if x >= 60 {
				c = color.RGBA{B: 255, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestPlaylistCoverPixelsPreserveAspectCropAndBGRA(t *testing.T) {
	data := coverFixture(t)
	for _, tc := range []struct {
		name      string
		source    PlaylistCoverSource
		blue, red byte
	}{
		{"poster left", PlaylistCoverSource{CropAspect: 0.5, CropPosition: 0}, 0, 255},
		{"poster right", PlaylistCoverSource{CropAspect: 0.5, CropPosition: 1}, 255, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pixels, err := playlistCoverPixels(data, tc.source)
			if err != nil {
				t.Fatal(err)
			}
			if len(pixels) != playlistCoverSize*playlistCoverSize*4 {
				t.Fatal("incorrect bitmap size")
			}
			center := (64*playlistCoverSize + 64) * 4
			if pixels[center] != tc.blue || pixels[center+2] != tc.red || pixels[center+3] != 255 {
				t.Fatalf("incorrect BGRA crop: %v", pixels[center:center+4])
			}
			if pixels[(64*playlistCoverSize)*4+3] != 0 {
				t.Fatal("poster was stretched instead of letterboxed")
			}
		})
	}
	_, err := playlistCoverPixels([]byte("not an image"), PlaylistCoverSource{})
	if err == nil {
		t.Fatal("invalid image accepted")
	}
	huge := append([]byte(nil), data...)
	binary.BigEndian.PutUint32(huge[16:20], 9000)
	binary.BigEndian.PutUint32(huge[20:24], 9000)
	binary.BigEndian.PutUint32(huge[29:33], crc32.ChecksumIEEE(huge[12:29]))
	if _, err := playlistCoverPixels(huge, PlaylistCoverSource{}); err == nil {
		t.Fatal("oversized dimensions accepted")
	}
}

func TestPlaylistCoversLazyDeduplicationAndCleanup(t *testing.T) {
	data := coverFixture(t)
	sourcePath := filepath.Join(t.TempDir(), "中文 cover.png")
	if err := os.WriteFile(sourcePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	var loads atomic.Int64
	resolve := func(context.Context) (PlaylistCoverSource, error) {
		loads.Add(1)
		return PlaylistCoverSource{Path: sourcePath}, nil
	}
	replies := make(chan map[string]json.RawMessage, 10)
	original := dialMPVIPCOverride
	dialMPVIPCOverride = func(_ string, _ time.Duration) (io.ReadWriteCloser, error) {
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			var request ipcRequest
			if json.NewDecoder(server).Decode(&request) != nil {
				return
			}
			var reply map[string]json.RawMessage
			if len(request.Command) == 5 {
				_ = json.Unmarshal([]byte(request.Command[4].(string)), &reply)
			}
			replies <- reply
			_ = json.NewEncoder(server).Encode(ipcResponse{Error: "success", RequestID: request.RequestID})
		}()
		return client, nil
	}
	c := newPlaylistCovers("test", map[string]PlaylistCoverResolver{"a": resolve, "b": resolve, "missing": nil})
	t.Cleanup(func() { c.cancel(); <-c.done; dialMPVIPCOverride = original })
	request := func(generation string, serial int64, keys ...string) {
		t.Helper()
		raw, _ := json.Marshal(playlistCoverRequest{Generation: generation, Serial: serial, Keys: keys})
		c.request(raw)
	}
	if loads.Load() != 0 {
		t.Fatal("covers loaded before viewport requested them")
	}
	request("stale generation", 1, "a")
	if c.latest.Serial != 0 {
		t.Fatal("stale playlist accepted")
	}
	request(c.generation, 1, "a", "a", "b", "missing", "unknown")
	var result map[string]json.RawMessage
	select {
	case result = <-replies:
	case <-time.After(5 * time.Second):
		t.Fatal("no artwork reply")
	}
	if loads.Load() != 2 || len(result) != 3 || string(result["missing"]) != "false" {
		t.Fatalf("incorrect deduplication/results: %d %v", loads.Load(), result)
	}
	var first, second playlistCoverImage
	_ = json.Unmarshal(result["a"], &first)
	_ = json.Unmarshal(result["b"], &second)
	if first.Path == "" || first.Path != second.Path {
		t.Fatal("identical image decoded more than once")
	}
	if first.Width != playlistCoverSize || first.Height != playlistCoverSize {
		t.Fatal("invalid image dimensions")
	}
	if info, err := os.Stat(first.Path); err != nil || info.Size() != playlistCoverSize*playlistCoverSize*4 {
		t.Fatalf("invalid bitmap file: %v", err)
	}
	request(c.generation, 2, "a")
	select {
	case <-replies:
	case <-time.After(time.Second):
		t.Fatal("cached reply missing")
	}
	if loads.Load() != 2 {
		t.Fatal("cached artwork reloaded")
	}
	c.cancel()
	<-c.done
	if _, err := os.Stat(first.Path); !os.IsNotExist(err) {
		t.Fatal("temporary bitmap was not cleaned up")
	}
	if original, err := os.ReadFile(sourcePath); err != nil || !bytes.Equal(original, data) {
		t.Fatal("source image was modified")
	}
}

func TestPlaylistCoverRequestsCoalesceAndCancel(t *testing.T) {
	started := make(chan struct{})
	c := newPlaylistCovers("unused", map[string]PlaylistCoverResolver{"one": func(ctx context.Context) (PlaylistCoverSource, error) {
		close(started)
		<-ctx.Done()
		return PlaylistCoverSource{}, ctx.Err()
	}})
	raw, _ := json.Marshal(playlistCoverRequest{Generation: c.generation, Serial: 2, Keys: []string{"one"}})
	c.request(raw)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("resolver not started")
	}
	raw, _ = json.Marshal(playlistCoverRequest{Generation: c.generation, Serial: 1, Keys: []string{"one"}})
	c.request(raw)
	if c.latest.Serial != 2 {
		t.Fatal("older viewport replaced the latest request")
	}
	c.cancel()
	select {
	case <-c.done:
	case <-time.After(time.Second):
		t.Fatal("shutdown waited for artwork")
	}
}

func TestDetachedPlayerCoverCleanup(t *testing.T) {
	var events *playlistEvents
	events.setPlaylistCovers(nil)
}
