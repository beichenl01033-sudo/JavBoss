package ltx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"javboss/internal/util"
)

var ErrSharedSidecars = errors.New("same-stem videos may share sidecars")

// RenameFamily moves a video and only unambiguously same-stem sidecars in its
// directory. It never copies media or overwrites a target; source entries must
// be regular files rather than symlinks.
// Undo is used if the database update fails; it also refuses to overwrite files.
func RenameFamily(oldPath, newPath string) (func() error, error) {
	return renameFamily(oldPath, newPath, moveExclusive)
}

func renameFamily(oldPath, newPath string, move func(string, string) error) (func() error, error) {
	if filepath.Dir(oldPath) != filepath.Dir(newPath) || !strings.EqualFold(filepath.Ext(oldPath), filepath.Ext(newPath)) {
		return nil, errors.New("LTX rename must keep directory and file extension")
	}
	oldStem := strings.TrimSuffix(filepath.Base(oldPath), filepath.Ext(oldPath))
	newStem := strings.TrimSuffix(filepath.Base(newPath), filepath.Ext(newPath))
	moves := [][2]string{{oldPath, newPath}}
	sharedStem := false
	subtitle := map[string]bool{".srt": true, ".ass": true, ".ssa": true, ".vtt": true, ".sub": true, ".idx": true, ".sup": true}
	entries, err := os.ReadDir(filepath.Dir(oldPath))
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == filepath.Base(oldPath) || !strings.HasPrefix(name, oldStem) {
			continue
		}
		suffix := strings.TrimPrefix(name, oldStem)
		ext := strings.ToLower(filepath.Ext(name))
		if strings.TrimSuffix(name, filepath.Ext(name)) == oldStem && !subtitle[ext] && util.IsVideoCandidate(filepath.Join(filepath.Dir(oldPath), name)) {
			sharedStem = true
		}
		isSidecar := strings.HasPrefix(suffix, ".") && (subtitle[ext] || strings.EqualFold(suffix, ".nfo"))
		if strings.HasPrefix(suffix, "-poster.") || strings.HasPrefix(suffix, "-fanart.") {
			isSidecar = ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp"
		}
		if isSidecar {
			moves = append(moves, [2]string{filepath.Join(filepath.Dir(oldPath), name), filepath.Join(filepath.Dir(oldPath), newStem+suffix)})
		}
	}
	if sharedStem && len(moves) > 1 {
		return nil, ErrSharedSidecars
	}
	for _, pair := range moves {
		info, err := os.Lstat(pair[0])
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, errors.New("cannot rename non-regular media or sidecar")
		}
		if _, err := os.Lstat(pair[1]); err == nil {
			return nil, fmt.Errorf("target already exists: %w", os.ErrExist)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	completed := 0
	undo := func() error {
		var failures []error
		for i := completed - 1; i >= 0; i-- {
			if err := move(moves[i][1], moves[i][0]); err != nil {
				failures = append(failures, err)
			}
		}
		return errors.Join(failures...)
	}
	for _, pair := range moves {
		if err := move(pair[0], pair[1]); err != nil {
			return nil, errors.Join(err, undo())
		}
		completed++
	}
	return undo, nil
}
