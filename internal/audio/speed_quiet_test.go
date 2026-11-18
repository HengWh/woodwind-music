//go:build woodwind_quiet

package audio

import (
	"math"
	"strconv"
	"testing"
)

// Mix a synthetic tone offline, without ever starting an output device.
// Tempo changes source time, while the rendered frequency stays at 440 Hz.
func TestPlaybackSpeedPreservesPitchAndSourceClock(t *testing.T) {
	const hz = 440
	samples := make([]float32, sampleRate*4)
	for i := range samples {
		samples[i] = float32(0.3 * math.Sin(2*math.Pi*hz*float64(i)/sampleRate))
	}
	for _, speed := range []float64{0.5, 0.75, 1, 1.25, 1.5, 1.75, 2} {
		t.Run(strconv.FormatFloat(speed, 'f', -1, 64), func(t *testing.T) {
			p := NewSound(sampleRate, 1, samples).NewPlayer().SetPreservesPitch(true).SetSpeed(speed)
			defer p.Close()
			// Render this voice alone; Play would start the shared output clock.
			v := p.v
			v.playing = true
			out := make([]float32, sampleRate*2)
			v.mix(out, 1, 0, 0)
			sourceSeconds := v.at() / sampleRate
			if math.Abs(sourceSeconds-speed) > 0.07 {
				t.Fatalf("source clock %.3fs at speed %.2f", sourceSeconds, speed)
			}
			crossings := 0
			for i := sampleRate/2*2 + 2; i < len(out); i += 2 {
				if out[i-2] <= 0 && out[i] > 0 {
					crossings++
				}
			}
			frequency := float64(crossings) * 2
			if math.Abs(frequency-hz) > 6 {
				t.Fatalf("pitch changed to %.1f Hz at speed %.2f", frequency, speed)
			}
		})
	}
}
