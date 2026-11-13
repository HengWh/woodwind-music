//go:build !windows

package nowplaying

// Initialize and Shutdown delimit the desktop integration's process lifetime.
func Initialize() error { return nil }
func Shutdown()         { Clear() }
