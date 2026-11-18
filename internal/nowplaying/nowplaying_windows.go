//go:build windows && !woodwind_quiet

package nowplaying

import (
	"github.com/nsgo-dev/nsgo/core"
	"time"

	"github.com/HengWh/woodwind-music/internal/winshell"
)

func Initialize() error {
	winshell.SetCommandHandler(func(cmd int, pos time.Duration) {
		if fn := OnCommand; fn != nil {
			fn(Command(cmd), pos)
		}
	})
	return winshell.Start(core.App().ID)
}

func Shutdown() { winshell.Stop(); winshell.SetCommandHandler(nil) }

func Set(info Info) {
	winshell.SetMedia(winshell.MediaInfo{Title: info.Title, Artist: info.Artist, Album: info.Album,
		Artwork: info.Artwork, Duration: info.Duration, Position: info.Position, Playing: info.Playing})
}
func Position(pos time.Duration, playing bool, _ float64) { winshell.MoveMedia(pos, playing) }
func Clear()                                              { winshell.ClearMedia() }
