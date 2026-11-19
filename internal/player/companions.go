package player

import "github.com/HengWh/woodwind-music/internal/lyrics"

// Completion follows actual playback (including resuming a restored session),
// and never reloads audio. The generation check discards a previous song's UI
// result while allowing its successfully downloaded companions to stay cached.
func (p *Player) completeLocal() {
	if p.CompleteLocal == nil || p.track == nil || p.track.LocalOnly || (p.track.Source != "local" && p.track.Source != "") || p.completeGen == p.gen {
		return
	}
	p.completeGen = p.gen
	gen, t, complete := p.gen, *p.track, p.CompleteLocal
	go func() {
		updated, words := complete(t)
		p.post(func() {
			if p.closed || p.gen != gen || p.track == nil {
				return
			}
			changed := false
			if p.track.Album.ImageURL == "" && updated.Album.ImageURL != "" {
				current := *p.track
				current.Album.ImageURL = updated.Album.ImageURL
				p.track = &current
				for i := range p.queue {
					if p.queue[i].Source == t.Source && p.queue[i].ID == t.ID {
						p.queue[i].Album.ImageURL = updated.Album.ImageURL
					}
				}
				p.loadCover(gen, &current)
				changed = true
			}
			if len(p.lyrics) == 0 && words != nil {
				p.lyrics = lyrics.FromSource(words)
				p.lyricsLoaded = true
				changed = len(p.lyrics) > 0 || changed
			}
			if changed {
				p.sync()
			}
		})
	}()
}
