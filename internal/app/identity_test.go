package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppDataDirCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, current, legacy, defaultDir, want string
	}{
		{"current wins", "/new", "/legacy", "/existing", "/new"},
		{"legacy override", "", "/legacy", "/existing", "/legacy"},
		{"existing data stays", "", "", "/existing", "/existing"},
		{"temporary fallback", "", "", "", filepath.Join(os.TempDir(), "woodwind-music")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("WOODWIND_MUSIC_DATA_DIR", tc.current)
			t.Setenv("NSGO_MUSIC_DATA_DIR", tc.legacy)
			if got := appDataDir(tc.defaultDir); got != tc.want {
				t.Fatalf("appDataDir = %q, want %q", got, tc.want)
			}
		})
	}
}
