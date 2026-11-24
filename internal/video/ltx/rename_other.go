//go:build !darwin && !linux && !windows

package ltx

import "errors"

func moveExclusive(string, string) error {
	return errors.New("exclusive rename is unsupported on this platform")
}
