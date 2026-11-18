package app

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/player"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestPlaybackSpeedMenusKeepLyricsMounted(t *testing.T) {
	for _, tc := range []struct {
		name   string
		width  core.Points
		button string
		bar    bool
	}{
		{"desktop bar", 1180, "bar-speed", true},
		{"desktop lyrics", 1180, "np-speed", false},
		{"phone lyrics", 390, "phone-speed", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", t.TempDir())
			var a *app
			ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				a = newApp(cx)
				reviewTrack(a)
				if tc.bar {
					a.closeNowPlaying()
				}
				return a.view(cx)
			}, nsgotest.Size(tc.width, 844))
			ui.WaitUntil(2*time.Second, func() bool { return !a.homeBusy })
			ui.Advance(time.Second)
			if tc.width < 600 {
				ui.Press("id:np-cover")
			}
			var lyrics *nsgotest.View
			var offset core.Point
			if !tc.bar {
				ui.Scroll("id:lyrics-list", 200)
				lyrics = ui.Find("id:lyrics-list")
				offset = lyrics.Offset
			}
			button := ui.Find("id:" + tc.button)
			position, playing := a.pl.Position(), a.pl.Playing()
			for _, speed := range []float64{1.5, 0.5, 1} {
				ui.Press("id:" + tc.button)
				ui.Press("id:" + tc.button + "-" + speedLabel(speed))
				ui.Advance(300 * time.Millisecond)
				if a.pl.Speed() != speed || a.pl.Position() != position || a.pl.Playing() != playing {
					t.Fatal("menu changed transport instead of speed")
				}
				if ui.Find("id:"+tc.button) != button || ui.Find("id:"+tc.button+"-value").Text != speedLabel(speed) {
					t.Fatal("speed label did not update in place")
				}
				if lyrics != nil && (ui.Find("id:lyrics-list") != lyrics || lyrics.Offset != offset) {
					t.Fatal("speed selection recreated or scrolled lyrics")
				}
				data, err := os.ReadFile(a.sessionPath())
				if err != nil {
					t.Fatal(err)
				}
				var session player.Session
				if err := json.Unmarshal(data, &session); err != nil {
					t.Fatal(err)
				}
				if session.Speed != speed {
					t.Fatal("menu did not save speed")
				}
			}
		})
	}
}

func TestPlaybackSpeedFitsSmallPlayer(t *testing.T) {
	for _, size := range []core.Size{{Width: 320, Height: 568}, {Width: 390, Height: 844}, {Width: 1180, Height: 600}} {
		t.Run(speedLabel(float64(size.Width)), func(t *testing.T) {
			t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", t.TempDir())
			var a *app
			ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				a = newApp(cx)
				reviewTrack(a)
				return a.view(cx)
			}, nsgotest.Size(size.Width, size.Height), nsgotest.Environment(core.Environment{Locale: "zh-CN", ReduceMotion: true}))
			ui.WaitUntil(2*time.Second, func() bool { return !a.homeBusy })
			ui.Advance(time.Second)
			id := "np-speed"
			if size.Width < 720 {
				id = "phone-speed"
			}
			b := reviewBounds(ui.Find("id:" + id))
			if b.Origin.X < 0 || b.Origin.Y < 0 || b.Origin.X+b.Size.Width > size.Width || b.Origin.Y+b.Size.Height > size.Height {
				t.Fatalf("speed control outside viewport: %+v, viewport %+v", b, size)
			}
			ui.Press("id:" + id)
			for _, speed := range player.Speeds() {
				b := reviewBounds(ui.Find("id:" + id + "-" + speedLabel(speed)))
				if b.Origin.Y < 0 || b.Origin.Y+b.Size.Height > size.Height {
					t.Fatalf("menu option %v outside viewport: %+v", speed, b)
				}
			}
		})
	}
}
