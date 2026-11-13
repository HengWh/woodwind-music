package app

import (
	"encoding/base64"
	"time"

	"github.com/HengWh/woodwind-music/internal/player"
)

// androidMedia talks to the application's Android service through NSGo's
// public host channel. core.Video remains the only audio player.
type androidMedia struct {
	info        player.NowPlaying
	resumeTrack string
}

func (a *app) setupAndroidMedia() {
	if !a.video.managed {
		return
	}
	a.androidMedia = &androidMedia{}
	player.System.Set = func(info player.NowPlaying) {
		a.androidMedia.info = info
		a.publishAndroidMedia(true)
	}
	player.System.Position = func(pos time.Duration, playing bool) {
		a.androidMedia.info.Position, a.androidMedia.info.Playing = pos, playing
		a.publishAndroidMedia(false)
	}
	player.System.Clear = func() { a.cx.PostHost("playback.update", map[string]any{"hasTrack": false}) }
	a.cx.OnHostEvent("android.lifecycle", func(data any) {
		if event, ok := data.(map[string]any); ok {
			if zone, ok := event["timeZone"].(string); ok && zone != "" && (a.location == nil || a.location.String() != zone) {
				a.location = deviceLocation(zone)
				a.bump()
			}
		}
		a.saveSession()
	})
	a.cx.OnHostEvent("android.media.command", func(data any) {
		m, ok := data.(map[string]any)
		if !ok {
			return
		}
		command, _ := m["command"].(string)
		if command != "focus-pause" && command != "focus-resume" {
			a.cancelFocusResume()
		}
		switch command {
		case "play":
			if !a.pl.Playing() {
				a.pl.TogglePlay()
			}
		case "pause", "stop":
			if a.pl.Playing() {
				a.pl.TogglePlay()
			}
		case "toggle":
			a.pl.TogglePlay()
		case "next":
			a.pl.Next()
		case "previous":
			a.pl.Prev()
		case "seek":
			if ms, ok := m["position"].(float64); ok {
				a.pl.Seek(time.Duration(ms) * time.Millisecond)
			}
		case "focus-pause":
			if a.pl.Playing() {
				a.androidMedia.resumeTrack = a.focusTrack()
				a.pl.TogglePlay()
			}
		case "focus-resume":
			if a.androidMedia.resumeTrack != "" && a.androidMedia.resumeTrack == a.focusTrack() && !a.pl.Playing() {
				a.pl.TogglePlay()
			}
			a.cancelFocusResume()
		}
		a.saveSession()
	})
}

func (a *app) cancelFocusResume() {
	if a.androidMedia != nil {
		a.androidMedia.resumeTrack = ""
	}
}

func (a *app) focusTrack() string {
	if t := a.pl.Current(); t != nil {
		return t.Source + "\x00" + t.ID
	}
	return ""
}

func (a *app) publishAndroidMedia(artwork bool) {
	s := a.androidMedia.info
	data := map[string]any{
		"hasTrack": a.pl.Current() != nil, "title": s.Title, "artist": s.Artist, "album": s.Album,
		"position": a.pl.Position().Milliseconds(), "duration": a.pl.Duration().Milliseconds(),
		"playing": a.pl.Playing(), "loading": a.pl.Loading(), "error": a.pl.Err(),
	}
	if artwork {
		data["artwork"] = base64.StdEncoding.EncodeToString(s.Artwork)
	}
	a.cx.PostHost("playback.update", data)
}
