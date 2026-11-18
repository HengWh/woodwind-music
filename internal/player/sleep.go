package player

import "time"

// SleepAfter pauses playback after elapsed time, including time spent paused
// or buffering. A nonpositive duration cancels it without changing playback.
// It belongs to this running player, never the saved session.
func (p *Player) SleepAfter(d time.Duration) {
	if p.closed {
		return
	}
	p.cancelSleepTimer()
	if d > 0 {
		p.sleepUntil = p.now().Add(d)
		gen := p.sleepGen
		p.sleepTimer = time.AfterFunc(d, func() {
			p.post(func() { p.expireSleepTimer(gen) })
		})
	}
	if p.SleepTimerChanged != nil {
		p.SleepTimerChanged()
	}
}

// SleepRemaining returns zero when no timer is set or its deadline has passed.
func (p *Player) SleepRemaining() time.Duration {
	if p.sleepUntil.IsZero() {
		return 0
	}
	return max(0, p.sleepUntil.Sub(p.now()))
}

func (p *Player) cancelSleepTimer() {
	p.sleepGen++
	if p.sleepTimer != nil {
		p.sleepTimer.Stop()
		p.sleepTimer = nil
	}
	p.sleepUntil = time.Time{}
}

func (p *Player) expireSleepTimer(gen int) {
	if p.closed || gen != p.sleepGen || p.sleepUntil.IsZero() {
		return
	}
	p.cancelSleepTimer()
	// A late radio feed or end event must not start the next song after sleep.
	p.awaiting = false
	if p.playing {
		switch {
		case p.remote != nil && p.remote.started, p.native != nil, p.out != nil:
			p.TogglePlay()
		default:
			// A load (including a remote play request) observes this intent
			// when it finishes. Do not retry an unavailable song here.
			p.playing = false
			p.sync()
		}
	}
	if p.SleepTimerChanged != nil {
		p.SleepTimerChanged()
	}
	if p.SleepTimerExpired != nil {
		p.SleepTimerExpired()
	}
}
