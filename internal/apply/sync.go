package apply

import (
	"fmt"
	"os"
	"path/filepath"
)

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync directory %s: %w", path, err)
	}
	return nil
}

func makeDirs(path string, mode os.FileMode) error {
	parent, _, err := existingParent(path)
	if err != nil {
		return err
	}
	if parent == path {
		// Already a directory, so there is nothing to create or persist.
		return nil
	}
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}
	// Persist newly created ancestors before a cross-volume move can remove
	// its source; syncing only the final directory would leave these unprotected.
	for path != parent {
		path = filepath.Dir(path)
		if err := syncDir(path); err != nil {
			return err
		}
	}
	return nil
}
