// Package ltx identifies explicit filename labels, not restoration quality.
package ltx

import (
	"path/filepath"
	"regexp"
	"strings"
)

var words = regexp.MustCompile(`[A-Za-z0-9]+`)

// Marked only examines the filename; an LTX parent directory is not evidence.
func Marked(filename string) bool {
	filename = filepath.Base(strings.ReplaceAll(filename, `\`, "/"))
	filename = strings.TrimSuffix(filename, filepath.Ext(filename))
	for _, word := range words.FindAllString(filename, -1) {
		if strings.EqualFold(word, "ltx") {
			return true
		}
	}
	return false
}
