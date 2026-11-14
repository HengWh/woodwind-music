package app

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/player"
)

func TestPlaybackModeKeepsPlayerMounted(t *testing.T) {
	for _, tc := range []struct {
		name   string
		width  core.Points
		bar    bool
		lyrics bool
		button string
		stable []string
	}{
		{"desktop bar", 1180, true, false, "id:playback-mode", []string{"id:bar-open", "id:bar-open-title", "id:play-pause", "id:progress"}},
		{"desktop lyrics", 1180, false, true, "id:np-controls >> id:playback-mode", []string{"id:np-cover", "id:np-title", "id:lyrics-list", "id:np-controls >> id:progress", "id:np-controls >> id:play-pause"}},
		{"phone cover", 390, false, false, "id:np-playback-mode", []string{"id:phone-now-playing", "id:np-cover", "id:np-title", "id:np-play-pause", "id:progress"}},
		{"phone lyrics", 390, false, true, "id:np-playback-mode", []string{"id:phone-now-playing", "id:lyrics-list", "id:np-title", "id:np-play-pause", "id:progress"}},
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
			if tc.width < 600 && tc.lyrics {
				ui.Press("id:np-cover")
			}
			if tc.lyrics {
				ui.Scroll("id:lyrics-list", 200)
			}
			stable := map[string]*nsgotest.View{tc.button: ui.Find(tc.button)}
			for _, selector := range tc.stable {
				stable[selector] = ui.Find(selector)
			}
			button := stable[tc.button]
			glyph := button.Children[0]
			previousDrawing := glyph.Drawing
			var lyricOffset core.Point
			if tc.lyrics {
				lyricOffset = stable["id:lyrics-list"].Offset
			}
			colors := func() map[core.Color]bool {
				out := map[core.Color]bool{}
				for _, op := range glyph.Ops {
					if op.Color != 0 {
						out[core.Color(op.Color)] = true
					}
				}
				return out
			}
			originalColors := colors()
			if len(originalColors) == 0 {
				t.Fatal("mode icon has no drawing color")
			}
			for _, want := range []string{"Single-track Loop", "Shuffle", "List Loop"} {
				ui.Press(tc.button)
				if button.Props.Label != i18n.T(want) || button.Children[0] != glyph {
					t.Fatal("mode label or icon did not update in place")
				}
				if glyph.Drawing == previousDrawing {
					t.Fatal("mode change did not redraw the icon")
				}
				previousDrawing = glyph.Drawing
				if !maps.Equal(colors(), originalColors) || button.Props.Selected {
					t.Fatal("mode changed the icon's color or marked the button selected")
				}
				for selector, view := range stable {
					if ui.Find(selector) != view {
						t.Fatalf("changing playback mode recreated %s", selector)
					}
				}
				if tc.lyrics && ui.Find("id:lyrics-list").Offset != lyricOffset {
					t.Fatal("mode change reset the lyric scroll position")
				}
			}
		})
	}
}

func TestPlaybackModeRefreshesQueueWithoutRecreatingCover(t *testing.T) {
	t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		reviewTrack(a)
		tracks := append([]music.Track(nil), a.pl.Queue()...)
		tracks = append(tracks, music.Track{ID: "two", Name: "Second"}, music.Track{ID: "three", Name: "Third"}, music.Track{ID: "four", Name: "Fourth"})
		a.pl.Cue(tracks)
		a.pl.Hold(20 * time.Second)
		a.pl.SetPlaybackMode(player.ModeShuffle)
		order, _ := a.pl.PlayOrder()
		copy(order, []int{0, 3, 2, 1})
		a.npPane.Set(1)
		return a.view(cx)
	}, nsgotest.Size(1180, 844))
	ui.WaitUntil(2*time.Second, func() bool { return !a.homeBusy })
	ui.Advance(time.Second)
	cover := ui.Find("id:np-cover")
	if got := ui.Find("id:np-pane >> role:button >> nth=1").Props.Label; got != "Fourth" {
		t.Fatalf("initial shuffled queue: second track %q", got)
	}
	ui.Press("id:np-controls >> id:playback-mode")
	if got := ui.Find("id:np-pane >> role:button >> nth=1").Props.Label; got != "Second" {
		t.Fatalf("queue did not return to list order: second track %q", got)
	}
	if ui.Find("id:np-cover") != cover {
		t.Fatal("queue mode change recreated the cover")
	}
}

func TestPlaybackModeButtonAcrossLayouts(t *testing.T) {
	t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		reviewTrack(a)
		a.closeNowPlaying()
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	ui.WaitUntil(2*time.Second, func() bool { return !a.homeBusy })
	checkLabel := func(id, want string) {
		t.Helper()
		if got := ui.Find("id:" + id).Props.Label; got != i18n.T(want) {
			t.Fatalf("%s: label %q, want %q", id, got, i18n.T(want))
		}
	}
	checkSymmetry := func(mode, queue, play string) {
		t.Helper()
		left, right, center := ui.Find(mode), ui.Find(queue), ui.Find(play)
		lb, rb, cb := reviewBounds(left), reviewBounds(right), reviewBounds(center)
		if lb.Size != rb.Size || left.Style.CornerRadius != right.Style.CornerRadius || left.Children[0].Frame.Size != right.Children[0].Frame.Size {
			t.Fatal("playback mode and queue buttons have different sizes or shapes")
		}
		leftGap := cb.Origin.X + cb.Size.Width/2 - lb.Origin.X - lb.Size.Width/2
		rightGap := rb.Origin.X + rb.Size.Width/2 - cb.Origin.X - cb.Size.Width/2
		if leftGap-rightGap > 1 || rightGap-leftGap > 1 || lb.Origin.Y != rb.Origin.Y {
			t.Fatal("mode and queue buttons are not symmetric around play/pause")
		}
	}
	checkSymmetry("id:playback-mode", "id:playback-queue", "id:play-pause")
	if len(ui.Query("id:player-bar >> label:"+i18n.T("Up Next"))) != 1 {
		t.Fatal("playback bar has duplicate queue buttons")
	}
	for _, want := range []string{"List Loop", "Single-track Loop", "Shuffle", "List Loop"} {
		checkLabel("playback-mode", want)
		ui.Press("id:playback-mode")
	}
	// The full desktop player shares the mode selected in the bar.
	ui.Press("id:playback-queue")
	if !a.nowPlaying || a.npPane.GetUntracked() != 1 {
		t.Fatal("queue button did not open the desktop queue")
	}
	checkLabel("np-controls >> id:playback-mode", "Single-track Loop")
	checkSymmetry("id:np-controls >> id:playback-mode", "id:np-controls >> id:playback-queue", "id:np-controls >> id:play-pause")
	ui.Resize(390, 844)
	checkLabel("np-playback-mode", "Single-track Loop")
	checkSymmetry("id:np-playback-mode", "id:np-queue", "id:np-play-pause")
	if len(ui.Query("id:phone-now-playing >> id:np-queue")) != 1 {
		t.Fatal("phone player has duplicate queue buttons")
	}
	if ui.Exists("id:np-shuffle") || ui.Exists("id:np-repeat") {
		t.Fatal("phone still exposes independent shuffle/repeat controls")
	}
	ui.Press("id:np-playback-mode")
	checkLabel("np-playback-mode", "Shuffle")
	if !a.pl.Shuffle() || a.pl.Repeat() != player.RepeatAll || a.pl.Current().ID != "review" || a.pl.Position() != 20*time.Second {
		t.Fatal("mode button did not select shuffle while preserving playback")
	}
	b, err := os.ReadFile(a.sessionPath())
	if err != nil {
		t.Fatal(err)
	}
	var saved player.Session
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	if !saved.Shuffle || saved.Repeat != player.RepeatAll {
		t.Fatalf("mode was not saved: %+v", saved)
	}
}

func TestPlaybackModeRestoresLegacySessions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		shuffle bool
		repeat  player.RepeatMode
		want    player.PlaybackMode
	}{
		{"sequential", false, player.RepeatOff, player.ModeListLoop},
		{"list loop", false, player.RepeatAll, player.ModeListLoop},
		{"single loop", false, player.RepeatOne, player.ModeSingleLoop},
		{"shuffle", true, player.RepeatOff, player.ModeShuffle},
		{"shuffle loop", true, player.RepeatAll, player.ModeShuffle},
		{"shuffle single", true, player.RepeatOne, player.ModeSingleLoop},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", dir)
			s := player.Session{Queue: []music.Track{{ID: "saved", Name: "Saved"}}, Order: []int{0}, Shuffle: tc.shuffle, Repeat: tc.repeat}
			b, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "session.json"), b, 0o600); err != nil {
				t.Fatal(err)
			}
			var a *app
			nsgotest.Start(t, func(cx *core.Scope) core.Node {
				a = newApp(cx)
				return a.view(cx)
			}, nsgotest.Size(1180, 780))
			if a.pl.PlaybackMode() != tc.want || a.pl.Shuffle() != (tc.want == player.ModeShuffle) || a.pl.Repeat() == player.RepeatOff {
				t.Fatalf("legacy mode not normalized: shuffle=%v repeat=%v", a.pl.Shuffle(), a.pl.Repeat())
			}
		})
	}
}
