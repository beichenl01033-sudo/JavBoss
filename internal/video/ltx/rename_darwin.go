package ltx

import "golang.org/x/sys/unix"

func moveExclusive(oldPath, newPath string) error {
	return unix.RenamexNp(oldPath, newPath, unix.RENAME_EXCL)
}
