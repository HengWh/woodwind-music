//go:build !darwin

package nowplaying

import "time"

// Set, Position and Clear are no-ops where the system shows no media.
func Set(Info)                     {}
func Position(time.Duration, bool) {}
func Clear()                       {}
