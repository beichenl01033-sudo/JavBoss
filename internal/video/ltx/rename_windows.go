package ltx

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func longPath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(path, `\\?\`) {
		return path, nil
	}
	if strings.HasPrefix(path, `\\`) {
		return `\\?\UNC\` + strings.TrimPrefix(path, `\\`), nil
	}
	return `\\?\` + path, nil
}

func moveExclusive(oldPath, newPath string) error {
	oldPath, err := longPath(oldPath)
	if err != nil {
		return err
	}
	newPath, err = longPath(newPath)
	if err != nil {
		return err
	}
	old, err := windows.UTF16PtrFromString(oldPath)
	if err != nil {
		return err
	}
	next, err := windows.UTF16PtrFromString(newPath)
	if err != nil {
		return err
	}
	return windows.MoveFile(old, next)
}
