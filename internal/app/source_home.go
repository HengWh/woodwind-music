package app

import (
	"errors"
	"github.com/HengWh/woodwind-music/internal/music"
)

// A source switch may reuse its own last content, never another source's.
// Details are still loaded through source-scoped routes and current requests.
type sourceHomeSnapshot struct {
	home          *music.Browse
	recent, liked []music.Track
	mine          []music.Playlist
}

func (a *app) rememberSourceHome() {
	if a.sourceHomes == nil {
		a.sourceHomes = map[string]sourceHomeSnapshot{}
	}
	a.sourceHomes[a.source().Info().ID] = sourceHomeSnapshot{home: a.home, recent: a.recent, mine: a.mine, liked: a.liked}
}
func sourceLoadError(err error) string {
	if err == nil || errors.Is(err, music.ErrUnsupported) {
		return ""
	}
	return err.Error()
}
