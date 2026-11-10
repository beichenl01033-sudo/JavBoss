package mpv

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const playlistCoverRequestProperty = "user-data/javboss/playlist-cover-request"
const playlistCoverSize = 128
const maxPlaylistCoverBytes = 20 << 20

// PlaylistCoverSource contains cached artwork, never a video to extract frames from.
// Data is used by authenticated remote clients; credentials never enter MPV/Lua.
type PlaylistCoverSource struct {
	Path         string
	Data         []byte
	CropAspect   float64
	CropPosition float64
}

type PlaylistCoverResolver func(context.Context) (PlaylistCoverSource, error)

type playlistVisual struct {
	Key     string `json:"key"`
	VideoID int64  `json:"video_id"`
	Title   string `json:"title"`
}

type playlistCoverImage struct {
	Path   string `json:"path"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type playlistCoverRequest struct {
	Generation string   `json:"generation"`
	Serial     int64    `json:"serial"`
	Keys       []string `json:"keys"`
}

// One worker per player coalesces viewport requests. Old viewports and playlists
// cannot create unbounded goroutines or delay playback/event handling.
type playlistCovers struct {
	mu         sync.Mutex
	generation string
	endpoint   string
	sources    map[string]PlaylistCoverResolver
	latest     playlistCoverRequest
	wake       chan struct{}
	ctx        context.Context
	cancel     context.CancelFunc
	done       chan struct{}
}

var nextPlaylistCoverGeneration atomic.Uint64
var playlistCoverSlots = make(chan struct{}, 2)

func playlistVisualKey(item PlaylistItem) string {
	sum := sha256.Sum256([]byte(strconv.FormatInt(item.Options.VideoID, 10) + "\x00" + item.Path))
	return hex.EncodeToString(sum[:16])
}

func newPlaylistCovers(endpoint string, sources map[string]PlaylistCoverResolver) *playlistCovers {
	ctx, cancel := context.WithCancel(context.Background())
	c := &playlistCovers{
		generation: strconv.FormatUint(nextPlaylistCoverGeneration.Add(1), 10),
		endpoint:   endpoint, sources: sources, ctx: ctx, cancel: cancel,
		wake: make(chan struct{}, 1), done: make(chan struct{}),
	}
	go c.run()
	return c
}

func (c *playlistCovers) request(raw json.RawMessage) {
	var request playlistCoverRequest
	if len(raw) > 16384 || json.Unmarshal(raw, &request) != nil || request.Generation != c.generation || len(request.Keys) > 63 || request.Serial <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctx.Err() != nil || request.Serial <= c.latest.Serial {
		return
	}
	c.latest = request
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *playlistCovers) current(serial int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ctx.Err() == nil && c.latest.Serial == serial
}

func (c *playlistCovers) run() {
	defer close(c.done)
	var directory string
	defer func() {
		if directory != "" {
			_ = os.RemoveAll(directory) // Only the private MkdirTemp directory below.
		}
	}()
	cache := make(map[string]any)
	decoded := make(map[string]playlistCoverImage)
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.wake:
		}
		c.mu.Lock()
		request := c.latest
		c.mu.Unlock()
		results := make(map[string]any)
		for _, key := range request.Keys {
			if !c.current(request.Serial) {
				break
			}
			if cached, ok := cache[key]; ok {
				results[key] = cached
				continue
			}
			resolve, ok := c.sources[key]
			if !ok {
				continue // MPV may only ask about items in this playlist.
			}
			var result any = false
			if resolve != nil {
				result = loadPlaylistCover(c.ctx, resolve, &directory, decoded)
			}
			cache[key], results[key] = result, result
		}
		if len(results) > 0 && c.ctx.Err() == nil {
			data, _ := json.Marshal(results)
			_ = runIPCCommand(c.endpoint, []any{"script-message-to", "playlist_sidebar", "covers-ready", c.generation, string(data)})
		}
	}
}

func loadPlaylistCover(ctx context.Context, resolve PlaylistCoverResolver, directory *string, decoded map[string]playlistCoverImage) any {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	select {
	case playlistCoverSlots <- struct{}{}:
		defer func() { <-playlistCoverSlots }()
	case <-ctx.Done():
		return false
	}
	source, err := resolve(ctx)
	if err != nil || ctx.Err() != nil {
		return false
	}
	data, err := readPlaylistCover(source)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(append([]byte(fmt.Sprintf("%g:%g:", source.CropAspect, source.CropPosition)), data...))
	identity := hex.EncodeToString(sum[:])
	if cached, ok := decoded[identity]; ok {
		return cached
	}
	pixels, err := playlistCoverPixels(data, source)
	if err != nil || ctx.Err() != nil {
		return false
	}
	if *directory == "" {
		*directory, _ = os.MkdirTemp("", "javboss-playlist-covers-")
	}
	if *directory == "" {
		return false
	}
	path := filepath.Join(*directory, identity+".bgra")
	// Immutable files avoid races with MPV 0.41's mmap reader.
	if err := os.WriteFile(path, pixels, 0600); err != nil {
		return false
	}
	result := playlistCoverImage{Path: path, Width: playlistCoverSize, Height: playlistCoverSize}
	decoded[identity] = result
	return result
}

func readPlaylistCover(source PlaylistCoverSource) ([]byte, error) {
	if len(source.Data) > 0 {
		if len(source.Data) > maxPlaylistCoverBytes {
			return nil, fmt.Errorf("playlist cover exceeds size limit")
		}
		return source.Data, nil
	}
	file, err := os.Open(source.Path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxPlaylistCoverBytes {
		return nil, fmt.Errorf("invalid playlist cover file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxPlaylistCoverBytes+1))
	if len(data) > maxPlaylistCoverBytes {
		return nil, fmt.Errorf("playlist cover exceeds size limit")
	}
	return data, err
}

func playlistCoverPixels(data []byte, source PlaylistCoverSource) ([]byte, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 16_000_000 {
		return nil, fmt.Errorf("invalid playlist cover dimensions")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	if aspect := source.CropAspect; aspect > 0 && !math.IsNaN(aspect) && !math.IsInf(aspect, 0) {
		position := source.CropPosition
		if math.IsNaN(position) || math.IsInf(position, 0) {
			position = 1
		}
		position = min(1, max(0, position))
		width := min(bounds.Dx(), max(1, int(math.Round(float64(bounds.Dy())*aspect))))
		height := min(bounds.Dy(), max(1, int(math.Round(float64(bounds.Dx())/aspect))))
		left := bounds.Min.X + int(math.Round(float64(bounds.Dx()-width)*position))
		top := bounds.Min.Y + (bounds.Dy()-height)/2
		bounds = image.Rect(left, top, left+width, top+height)
	}
	scale := min(float64(playlistCoverSize)/float64(bounds.Dx()), float64(playlistCoverSize)/float64(bounds.Dy()))
	width, height := max(1, int(math.Round(float64(bounds.Dx())*scale))), max(1, int(math.Round(float64(bounds.Dy())*scale)))
	left, top := (playlistCoverSize-width)/2, (playlistCoverSize-height)/2
	canvas := image.NewRGBA(image.Rect(0, 0, playlistCoverSize, playlistCoverSize))
	draw.CatmullRom.Scale(canvas, image.Rect(left, top, left+width, top+height), img, bounds, draw.Src, nil)
	for index := 0; index < len(canvas.Pix); index += 4 {
		canvas.Pix[index], canvas.Pix[index+2] = canvas.Pix[index+2], canvas.Pix[index]
	}
	return canvas.Pix, nil
}
