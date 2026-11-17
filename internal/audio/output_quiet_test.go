//go:build woodwind_quiet

package audio

import (
	"testing"
	"time"
)

// The quiet output must remain null even while a loud source really advances.
// Muting the mixer alone would still open hardware and steal the audio device.
func TestQuietOutputAndPlaybackClock(t *testing.T) {
	dev, n, err := openDevice(LatencyPlayback.ahead())
	if err != nil {
		t.Fatal(err)
	}
	d, ok := dev.(*nullDevice)
	if !ok || n != nullFrames || !d.retry.IsZero() {
		t.Fatalf("quiet output must never open or retry hardware: %T, %d frames", dev, n)
	}
	dev.close()

	p := NewPlayer(&quietTone{})
	t.Cleanup(p.Close)
	p.Play()
	deadline := time.Now().Add(2 * time.Second)
	for p.Position() < 30*time.Millisecond && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if p.Position() < 30*time.Millisecond {
		t.Fatal("null output stopped the playback clock")
	}
	mu.Lock()
	buffered := held
	mu.Unlock()
	if buffered != 0 {
		t.Fatalf("null output reports %d frames buffered in hardware", buffered)
	}
}

type quietTone struct{}

func (*quietTone) SampleRate() int         { return sampleRate }
func (*quietTone) Channels() int           { return channels }
func (*quietTone) Len() int64              { return sampleRate }
func (*quietTone) SetPosition(int64) error { return nil }
func (*quietTone) Read(buf []float32) (int, error) {
	for i := range buf {
		buf[i] = 0.5
	}
	return len(buf), nil
}
