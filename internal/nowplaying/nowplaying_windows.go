//go:build windows

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

func Set(info Info)                            { winshell.SetMedia(winshell.MediaInfo(info)) }
func Position(pos time.Duration, playing bool) { winshell.MoveMedia(pos, playing) }
func Clear()                                   { winshell.ClearMedia() }
