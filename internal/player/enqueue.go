package player

import "github.com/HengWh/woodwind-music/internal/music"

// Enqueue appends without replacing or seeking the active queue.
func (p *Player) Enqueue(tracks []music.Track) {
	if p.closed || len(tracks) == 0 {
		return
	}
	if len(p.queue) == 0 {
		p.PlayTrack(tracks, 0)
		return
	}
	for _, t := range tracks {
		p.order = append(p.order, len(p.queue))
		p.queue = append(p.queue, t)
	}
	p.sync()
}
