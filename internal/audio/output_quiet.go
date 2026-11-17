//go:build woodwind_quiet

package audio

import "time"

// Quiet tests mix and advance normally without ever opening a system device.
func openDevice(time.Duration) (device, int, error) {
	return newNullDevice(false), nullFrames, nil
}
