package files

import (
	"errors"
	"os"
	"path/filepath"
)

// WriteFile creates parent directories and writes content to path.
func WriteFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

// WriteMarkerFile merges generated content into an Atlas marker block.
func WriteMarkerFile(path string, marker string, generated string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	merged := MergeMarkerBlock(string(existing), marker, generated)
	return WriteFile(path, []byte(merged))
}
