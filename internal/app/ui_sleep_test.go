package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/player"
)

func TestSleepTimerControlsAcrossLayouts(t *testing.T) {
	for _, width := range []core.Points{1180, 390} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", t.TempDir())
			var a *app
			ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				a = newApp(cx)
				reviewTrack(a)
				if width > 720 {
					a.closeNowPlaying()
				}
				return a.view(cx)
			}, nsgotest.Size(width, 844))
			ui.WaitUntil(2*time.Second, func() bool { return !a.homeBusy })
			button := "id:bar-sleep"
			if width < 720 {
				button = "id:np-sleep"
			}
			ui.Press(button)
			for _, minutes := range []int{15, 30, 45, 60, 90, 120} {
				if !ui.Exists(fmt.Sprintf("id:sleep-%d", minutes)) {
					t.Fatalf("missing preset %d", minutes)
				}
			}
			ui.Press("id:sleep-30")
			if a.sleepOpen.GetUntracked() || a.pl.SleepRemaining() < 29*time.Minute || a.pl.SleepRemaining() > 30*time.Minute {
				t.Fatal("preset did not start timer and close dialog")
			}
			if !strings.Contains(ui.Find(button).Props.Label, "30:00") {
				t.Fatal("timer button does not show remaining time")
			}
			ui.Press(button)
			ui.Type("id:sleep-minutes", "0")
			ui.Press("id:sleep-start")
			if !a.sleepOpen.GetUntracked() || a.sleepError.GetUntracked() == "" || a.pl.SleepRemaining() < 29*time.Minute {
				t.Fatal("invalid custom minutes changed the timer or lacked feedback")
			}
			ui.Clear("id:sleep-minutes")
			ui.Type("id:sleep-minutes", "7")
			ui.Press("id:sleep-start")
			if a.sleepOpen.GetUntracked() || a.pl.SleepRemaining() < 6*time.Minute || a.pl.SleepRemaining() > 7*time.Minute {
				t.Fatal("custom timer was not applied")
			}
			ui.Press(button)
			ui.Press("id:sleep-cancel")
			if a.pl.SleepRemaining() != 0 || !a.pl.Playing() || a.pl.Position() != 20*time.Second {
				t.Fatal("canceling the timer changed playback")
			}
			if ui.Find(button).Props.Label != i18n.T("Sleep Timer") {
				t.Fatal("canceling did not clear countdown")
			}
			if width > 720 {
				ui.Press("id:bar-open")
				if ui.Exists("id:np-sleep") {
					t.Fatal("desktop full player should not show a timer beside the close button")
				}
				ui.Press("id:np-close")
				if !ui.Exists("id:bar-sleep") {
					t.Fatal("closing desktop full player should restore the playback bar timer")
				}
			}
		})
	}
}

func TestSleepTimerExpirySavesAndCancelsFocusResume(t *testing.T) {
	t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		reviewTrack(a)
		return a.view(cx)
	}, nsgotest.Size(390, 844))
	ui.WaitUntil(2*time.Second, func() bool { return !a.homeBusy })
	a.post(func() {
		a.androidMedia = &androidMedia{resumeTrack: a.focusTrack()}
		a.radioStarting = "pending station"
		a.pl.SleepAfter(time.Millisecond)
	})
	ui.WaitUntil(time.Second, func() bool { return a.toast.GetUntracked() == i18n.T("Sleep timer ended. Playback paused.") })
	if a.pl.Playing() || a.androidMedia.resumeTrack != "" || a.radioStarting != "" {
		t.Fatal("sleep left playback or automatic resumption active")
	}
	data, err := os.ReadFile(a.sessionPath())
	if err != nil {
		t.Fatal(err)
	}
	var session player.Session
	if err := json.Unmarshal(data, &session); err != nil {
		t.Fatal(err)
	}
	if session.Position != 20000 || len(session.Queue) != 1 || session.Queue[0].ID != "review" {
		t.Fatal("sleep did not save queue and position")
	}
}

func TestSleepTimerPhoneSmallScreenAndKeyboard(t *testing.T) {
	ui, a := startPhone(t)
	ui.Resize(320, 568)
	env := core.Environment{OS: core.OSAndroid, Locale: "zh-CN", FontScale: 1.5, ReduceMotion: true, SafeArea: core.Edges{Top: 24, Bottom: 24}}
	ui.SetEnvironment(env)
	a.post(func() {
		reviewTrack(a)
		a.startSleep(1440)
	})
	ui.Settle()
	phoneWithin(t, ui, "np-sleep", 320, 568)
	ui.Press("id:np-sleep")
	ui.Scroll("id:sleep-scroll", 1000)
	phoneWithin(t, ui, "sleep-cancel", 320, 568)
	phoneWithin(t, ui, "sleep-start", 320, 568)
	env.KeyboardHeight = 280
	ui.SetEnvironment(env)
	ui.Scroll("id:sleep-scroll", 1000)
	b := reviewBounds(ui.Find("id:sleep-scroll"))
	if b.Origin.Y < 24 || b.Origin.Y+b.Size.Height > 568-280+1 {
		t.Fatalf("timer sheet covered by keyboard: %+v", b)
	}
	ui.Press("id:sleep-cancel")
	if a.pl.SleepRemaining() != 0 {
		t.Fatal("cancel action could not be reached with keyboard open")
	}
}

func TestSleepTimerCustomValidation(t *testing.T) {
	root := core.NewRoot(core.RootOptions{})
	defer root.Dispose()
	a := &app{sleepError: core.NewSignal(root, "")}
	for _, value := range []string{"", "-1", "0", "1.5", "1441", "99999999999999999999999", "abc"} {
		a.sleepMinutes = value
		a.startCustomSleep()
		if a.sleepError.GetUntracked() == "" {
			t.Fatalf("accepted invalid minutes %q", value)
		}
	}
}

func TestSleepTimerActiveBarFitsNarrowWindow(t *testing.T) {
	t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		reviewTrack(a)
		a.closeNowPlaying()
		a.pl.SleepAfter(24 * time.Hour)
		return a.view(cx)
	}, nsgotest.Size(880, 580), nsgotest.Environment(core.Environment{OS: core.OSMacOS}))
	if !ui.Exists("id:bar-desktop-lyrics") {
		t.Fatal("desktop lyrics button missing from the combined controls")
	}
	for _, width := range []core.Points{880, 1180} {
		ui.Resize(width, 580)
		timer := reviewBounds(ui.Find("id:bar-sleep"))
		queue := reviewBounds(ui.Find("id:playback-queue"))
		volume := reviewBounds(ui.Find("id:bar-volume"))
		if timer.Origin.X < queue.Origin.X+queue.Size.Width || volume.Origin.X+volume.Size.Width > width {
			t.Fatalf("active timer overlaps transport or volume at width %v: timer=%+v queue=%+v volume=%+v", width, timer, queue, volume)
		}
	}
}
