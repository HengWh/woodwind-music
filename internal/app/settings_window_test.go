package app

import (
	"sync"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// Unlike a settings view mounted in a single test surface, this exercises
// the desktop window lifetime through the real sidebar button and redraws.
func TestSettingsWindowSurvivesSidebarUpdates(t *testing.T) {
	t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", t.TempDir())
	var mu sync.Mutex
	var posts []func()
	root := core.NewRoot(core.RootOptions{
		Post:    func(fn func()) { mu.Lock(); posts = append(posts, fn); mu.Unlock() },
		Clock:   core.NewManualClock(time.Unix(0, 0)),
		Windows: &core.WindowOptions{Width: 1180, Height: 780},
	})
	t.Cleanup(root.Dispose)
	tree := core.NewTree(root, core.ApproxTextEngine{})
	var a *app
	tree.Mount(core.AppRoot(func(cx *core.Scope) {
		desktop.SetShutdownMode(cx, desktop.ShutdownOnExplicit)
		a = newApp(cx)
		cx.SetMainView(a.view)
	}))
	ids := map[string]core.NodeID{}
	flush := func() {
		for range 100 {
			mu.Lock()
			pending := posts
			posts = nil
			mu.Unlock()
			for _, fn := range pending {
				fn()
			}
			changed := len(pending) > 0
			for _, surface := range append([]*core.Tree{tree}, tree.Surfaces()...) {
				if surface.Closed() {
					surface.Forget()
					continue
				}
				if batch := surface.Commit(); batch != nil {
					changed = true
					for _, item := range batch.Items {
						if item.ViewProps != nil && item.ViewProps.TestID != "" {
							ids[item.ViewProps.TestID] = item.ID
						}
					}
				}
			}
			if !changed {
				return
			}
		}
		t.Fatal("the window did not settle")
	}
	flush()
	main := tree.Surfaces()[0]
	pressSettings := func() {
		t.Helper()
		id := ids["open-settings"]
		if id == 0 {
			t.Fatal("no settings button")
		}
		main.Dispatch(nsgotest.Event{Kind: nsgotest.EventPress, ID: id})
		flush()
		if a.settingsWin == nil || a.settingsWin.Closed() || !a.settingsWin.Visible() {
			t.Fatal("clicking Settings did not leave a visible settings window")
		}
	}
	pressSettings()
	first := a.settingsWin
	a.post(a.bump)
	flush()
	if first.Closed() {
		t.Fatal("a sidebar update closed the settings window")
	}
	first.Hide()
	pressSettings()
	if a.settingsWin != first || len(tree.Surfaces()) != 2 {
		t.Fatal("opening Settings again did not reuse its window")
	}
	// Closing with the settings window's shortcut must allow opening a fresh
	// window from the button, without retaining old surfaces.
	for _, surface := range tree.Surfaces() {
		if surface != main {
			surface.Dispatch(nsgotest.Event{Kind: nsgotest.EventKey, Text: "mod+w"})
		}
	}
	flush()
	if !first.Closed() {
		t.Fatal("the settings window did not close")
	}
	pressSettings()
	if a.settingsWin == first || len(tree.Surfaces()) != 2 {
		t.Fatal("Settings did not reopen as a new window")
	}
}
