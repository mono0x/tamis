package apply

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func validateTrash(source, dest string) error {
	if dest != "" {
		return nil
	}
	if !filepath.IsAbs(source) || strings.ContainsRune(source, 0) || !utf8.ValidString(source) {
		return fmt.Errorf("invalid native trash path %q", source)
	}
	// The library loads native APIs only during Move; availability is checked there.
	return nil
}
