package agents

import "os"

// fileExists keeps adapter detection tolerant of missing optional agent files.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// dirExists keeps adapter detection tolerant of missing optional agent directories.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
