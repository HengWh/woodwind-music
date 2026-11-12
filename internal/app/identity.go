package app

import (
	"os"
	"path/filepath"
)

// appDataDir keeps existing installations on their stable NSGo data
// directory while allowing both the new and legacy environment overrides.
func appDataDir(defaultDir string) string {
	for _, key := range []string{"WOODWIND_MUSIC_DATA_DIR", "NSGO_MUSIC_DATA_DIR"} {
		if dir := os.Getenv(key); dir != "" {
			return dir
		}
	}
	if defaultDir != "" {
		return defaultDir
	}
	return filepath.Join(os.TempDir(), "woodwind-music")
}
