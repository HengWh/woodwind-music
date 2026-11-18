package player

// RefreshStored updates presentation without reloading or seeking audio.
func (p *Player) RefreshStored() {
	if p.Library == nil || p.track == nil {
		return
	}
	r, ok := p.Library.Get(*p.track)
	if !ok || len(r.Variants) == 0 {
		return
	}
	local := p.Library.Track(r)
	local.LocalOnly = p.track.LocalOnly
	coverChanged := local.Album.ImageURL != p.track.Album.ImageURL
	p.track = &local
	if p.at >= 0 && p.at < len(p.order) {
		p.queue[p.order[p.at]] = local
	}
	if coverChanged {
		p.loadCover(p.gen, &local)
	}
	if len(p.lyrics) == 0 && r.Lyrics != nil {
		p.loadLyrics(p.gen, nil, local)
	}
}
