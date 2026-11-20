package app

import (
	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
)

// allLiked returns a complete snapshot or an error, never a partial collection.
// Offsets count raw plugin results, independently of duplicate filtering.
func allLiked(src music.Source) ([]music.Track, error) {
	var tracks []music.Track
	seen := map[string]bool{}
	for offset := 0; ; {
		page, err := src.Liked(offset)
		if err != nil {
			return nil, err
		}
		if page == nil {
			return nil, music.Fail(music.ErrUnavailable, i18n.T("Could not load all liked songs. Please try again."))
		}
		before := len(tracks)
		for _, tr := range page.Tracks {
			if tr.ID == "" || seen[tr.ID] {
				continue
			}
			seen[tr.ID] = true
			tr.Source = src.Info().ID
			tracks = append(tracks, tr)
		}
		if !page.More {
			return tracks, nil
		}
		// A next page must advance the collection, or the plugin could loop
		// forever while claiming that a partial snapshot is complete.
		if len(tracks) == before {
			return nil, music.Fail(music.ErrUnavailable, i18n.T("Could not load all liked songs. Please try again."))
		}
		offset += len(page.Tracks)
	}
}
