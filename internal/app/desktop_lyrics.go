package app

import (
	"time"

	"github.com/HengWh/woodwind-music/internal/desktoplyrics"
	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/lyrics"
	"github.com/nsgo-dev/nsgo/core"
)

type desktopLyricsSettings struct {
	Enabled     bool   `json:"enabled"`
	Locked      bool   `json:"locked"`
	FontSize    int    `json:"fontSize"`
	Color       string `json:"color"`
	Translation bool   `json:"translation"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
	Positioned  bool   `json:"positioned"`
}

func defaultDesktopLyricsSettings() desktopLyricsSettings {
	return desktopLyricsSettings{FontSize: 36, Color: "gold", Translation: true}
}

func (s *desktopLyricsSettings) normalize() {
	if s.FontSize < 24 || s.FontSize > 48 {
		s.FontSize = 36
	}
	switch s.Color {
	case "gold", "cyan", "pink":
	default:
		s.Color = "gold"
	}
}

func (a *app) supportsDesktopLyrics() bool {
	return a.cx.Env().OS() == core.OSMacOS || a.cx.Env().OS() == core.OSWindows
}

func (a *app) initDesktopLyrics() {
	a.openLyricWindow = desktoplyrics.Open
	a.cx.Cleanup(a.closeDesktopLyrics)
	if a.supportsDesktopLyrics() && a.set.DesktopLyrics.Enabled {
		a.post(a.showDesktopLyrics)
	}
}

func (a *app) toggleDesktopLyrics() { a.setDesktopLyricsEnabled(!a.set.DesktopLyrics.Enabled) }

func (a *app) setDesktopLyricsEnabled(enabled bool) {
	if !a.supportsDesktopLyrics() {
		return
	}
	a.set.DesktopLyrics.Enabled = enabled
	a.set.save()
	if enabled {
		a.post(a.showDesktopLyrics)
	} else {
		a.closeDesktopLyrics()
	}
	a.bump()
}

func (a *app) showDesktopLyrics() {
	if !a.supportsDesktopLyrics() || !a.set.DesktopLyrics.Enabled || a.lyricWindow != nil {
		return
	}
	w, err := a.openLyricWindow()
	if err != nil {
		a.set.DesktopLyrics.Enabled = false
		a.set.save()
		a.say(i18n.T("Desktop lyrics could not be opened."))
		a.bump()
		return
	}
	a.lyricWindow = w
	a.desktopLyricsBeat()
}

func (a *app) closeDesktopLyrics() {
	if a.lyricTimer != nil {
		a.lyricTimer()
		a.lyricTimer = nil
	}
	if a.lyricWindow != nil {
		a.lyricWindow.Close()
		a.lyricWindow = nil
	}
}

func (a *app) toggleDesktopLyricsLock() {
	a.set.DesktopLyrics.Locked = !a.set.DesktopLyrics.Locked
	a.saveDesktopLyricsStyle()
}

func (a *app) resetDesktopLyricsPosition() {
	a.set.DesktopLyrics.X, a.set.DesktopLyrics.Y = 0, 0
	a.set.DesktopLyrics.Positioned = false
	a.set.save()
	a.closeDesktopLyrics()
	if a.set.DesktopLyrics.Enabled {
		a.post(a.showDesktopLyrics)
	}
	a.bump()
}

func (a *app) saveDesktopLyricsStyle() {
	a.set.DesktopLyrics.normalize()
	a.set.save()
	if a.lyricWindow != nil {
		a.lyricWindow.Update(a.desktopLyricsFrame())
	}
	a.bump()
}

// The timer belongs to the app, so hiding or rebuilding the main view leaves
// this window and its playback updates alive. Closing cancels the timer.
func (a *app) desktopLyricsBeat() {
	if a.lyricWindow == nil {
		return
	}
	for _, event := range a.lyricWindow.Poll() {
		if event.Moved {
			a.set.DesktopLyrics.X, a.set.DesktopLyrics.Y = event.X, event.Y
			a.set.DesktopLyrics.Positioned = true
			a.set.save()
		}
		switch event.Action {
		case desktoplyrics.Hide:
			a.setDesktopLyricsEnabled(false)
			return
		case desktoplyrics.Lock:
			a.toggleDesktopLyricsLock()
		case desktoplyrics.Previous:
			a.pl.Prev()
		case desktoplyrics.TogglePlay:
			a.toggle()
		case desktoplyrics.Next:
			a.pl.Next()
		}
	}
	a.lyricWindow.Update(a.desktopLyricsFrame())
	delay := 200 * time.Millisecond
	if a.pl.Playing() {
		delay = 40 * time.Millisecond
	}
	a.cx.Run(func() { a.lyricTimer = a.cx.AfterFunc(delay, a.desktopLyricsBeat) })
}

func (a *app) desktopLyricsFrame() desktoplyrics.Frame {
	s := a.set.DesktopLyrics
	color := uint32(0xffd36b)
	switch s.Color {
	case "cyan":
		color = 0x63e6ff
	case "pink":
		color = 0xff92cd
	}
	f := desktoplyrics.Frame{FontSize: s.FontSize, Color: color, Locked: s.Locked, Playing: a.pl.Playing(), X: s.X, Y: s.Y, Positioned: s.Positioned}
	tr := a.pl.Current()
	if tr == nil {
		f.Words = []desktoplyrics.Word{{Text: i18n.T("Not Playing")}}
		return f
	}
	lines, loaded := a.pl.Lyrics()
	pos := a.pl.Position()
	i := lyrics.Index(lines, pos)
	if i < 0 {
		f.Words = []desktoplyrics.Word{{Text: tr.Name}}
		switch {
		case !loaded:
			f.Secondary = i18n.T("Loading…")
		case len(lines) == 0:
			f.Secondary = i18n.T("No lyrics. Enjoy the music.")
		default:
			f.Secondary = lines[0].Text
		}
		return f
	}
	line := lines[i]
	for _, word := range line.Words {
		progress := float64(0)
		if word.End <= word.Start {
			if pos >= word.Start {
				progress = 1
			}
		} else {
			progress = min(1, max(0, float64(pos-word.Start)/float64(word.End-word.Start)))
		}
		f.Words = append(f.Words, desktoplyrics.Word{Text: word.Text, Progress: progress})
	}
	if len(f.Words) == 0 {
		f.Words = []desktoplyrics.Word{{Text: line.Text}}
	}
	if s.Translation && line.Translation != "" {
		f.Secondary = line.Translation
	} else if i+1 < len(lines) {
		f.Secondary = lines[i+1].Text
	}
	return f
}
