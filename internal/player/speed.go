package player

import (
	"github.com/HengWh/woodwind-music/internal/music"
	"math"
)

// Speeds are the supported pitch-preserving playback rates.
func Speeds() []float64 { return []float64{0.5, 0.75, 1, 1.25, 1.5, 1.75, 2} }

func normalizedSpeed(speed float64) float64 {
	if math.IsNaN(speed) || math.IsInf(speed, 0) || speed < 0.5 || speed > 2 {
		return 1
	}
	return speed
}

// CanSetSpeed excludes sources that control a separate playback device.
func (p *Player) CanSetSpeed() bool {
	if p.remote != nil {
		return false
	}
	if p.track != nil {
		if src := p.sourceOf(p.track); src != nil && src.Info().Playback == music.PlaybackRemote {
			return false
		}
	}
	return true
}

// Speed is the effective playback rate. Old sessions default to normal speed.
func (p *Player) Speed() float64 {
	if !p.CanSetSpeed() {
		return 1
	}
	return normalizedSpeed(p.speed)
}

// SetSpeed changes tempo in place, retaining pitch, position and pause state.
func (p *Player) SetSpeed(speed float64) {
	if !p.CanSetSpeed() || normalizedSpeed(speed) != speed {
		return
	}
	p.speed = speed
	p.applySpeed()
	// Avoid rebuilding artwork or lyric lists for a tempo-only change.
	if p.track != nil && System.Position != nil {
		System.Position(p.Position(), p.playing)
	}
}

func (p *Player) applySpeed() {
	if p.out != nil {
		p.out.SetPreservesPitch(true).SetSpeed(p.Speed())
	}
	if p.Native != nil {
		p.Native.SetSpeed(p.Speed())
	}
}
