//go:build (android || (!darwin && !(linux && (amd64 || arm64)) && !(windows && (amd64 || arm64)) && !js)) && !woodwind_quiet

package audio

import "time"

func openDevice(ahead time.Duration) (device, int, error) { return nil, 0, errUnsupported }
