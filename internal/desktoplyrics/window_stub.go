//go:build woodwind_quiet || (!darwin && !windows)

package desktoplyrics

import "errors"

// Quiet tests must never open native windows, including this independent one.
func Open() (Window, error) { return nil, errors.New("desktop lyrics unavailable in this environment") }
