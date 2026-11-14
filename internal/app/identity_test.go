package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppDataDirCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, current, previous, legacy, defaultDir, want string
	}{
		{"current wins", "/new", "/previous", "/legacy", "/existing", "/new"},
		{"previous override", "", "/previous", "/legacy", "/existing", "/previous"},
		{"legacy override", "", "", "/legacy", "/existing", "/legacy"},
		{"existing data stays", "", "", "", "/existing", "/existing"},
		{"temporary fallback", "", "", "", "", filepath.Join(os.TempDir(), "wood-wind-music")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", tc.current)
			t.Setenv("WOODWIND_MUSIC_DATA_DIR", tc.previous)
			t.Setenv("NSGO_MUSIC_DATA_DIR", tc.legacy)
			if got := appDataDir(tc.defaultDir); got != tc.want {
				t.Fatalf("appDataDir = %q, want %q", got, tc.want)
			}
		})
	}
}
