package desktoplyrics

import (
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// Exercise the real NSGo surface protocol without a native host. No build tag
// substitutes for the implementation under test, and no device is opened.
type panelFixture struct {
	root       *core.Scope
	tree       *core.Tree
	window     *panelWindow
	ids        map[string]core.NodeID
	opened     []nsgotest.SurfaceConfig
	commands   []core.WindowCommand
	drawing    *core.Drawing
	background core.Color
	posts      []func()
}

func newPanelFixture(t *testing.T, os core.OS, initial Frame) *panelFixture {
	t.Helper()
	f := &panelFixture{ids: map[string]core.NodeID{}}
	f.root = core.NewRoot(core.RootOptions{Post: func(fn func()) { f.posts = append(f.posts, fn) }, Clock: core.NewManualClock(time.Unix(0, 0)), Windows: &core.WindowOptions{Width: 1180, Height: 780}})
	t.Cleanup(f.root.Dispose)
	f.tree = core.NewTree(f.root, core.ApproxTextEngine{})
	f.tree.SetEnvironment(core.Environment{OS: os, Displays: []core.DisplayInfo{display(0, 0, 1440, 900, 2, true)}})
	f.tree.Mount(core.AppRoot(func(cx *core.Scope) {
		w, err := Open(cx, initial)
		if err != nil {
			t.Fatal(err)
		}
		f.window = w.(*panelWindow)
	}))
	f.flush(t)
	return f
}

func (f *panelFixture) flush(t *testing.T) {
	t.Helper()
	for range 100 {
		posts := f.posts
		f.posts = nil
		for _, fn := range posts {
			fn()
		}
		changed := len(posts) > 0
		for _, tree := range append([]*core.Tree{f.tree}, f.tree.Surfaces()...) {
			if tree.Closed() {
				tree.Forget()
				continue
			}
			if batch := tree.Commit(); batch != nil {
				changed = true
				for _, item := range batch.Items {
					if item.Surface != nil {
						f.opened = append(f.opened, *item.Surface)
					}
					if item.Window != nil {
						f.commands = append(f.commands, *item.Window)
					}
					if item.ViewProps != nil && item.ViewProps.TestID != "" {
						f.ids[item.ViewProps.TestID] = item.ID
					}
					if item.Drawing != nil {
						f.drawing = item.Drawing
					}
					if item.ViewStyle != nil && item.ID == f.ids["desktop-lyrics-panel"] {
						f.background = item.ViewStyle.Background
					}
				}
			}
		}
		if !changed {
			return
		}
	}
	t.Fatal("panel did not settle")
}

func (f *panelFixture) surface() *core.Tree { return f.tree.Surfaces()[0] }

func TestPanelControlsPointerAndLifetime(t *testing.T) {
	for _, os := range []core.OS{core.OSMacOS, core.OSWindows} {
		t.Run(string(os), func(t *testing.T) {
			initial := Frame{Words: []Word{{Text: "你好", Progress: .5}}, Secondary: "Hello", FontSize: 36, Color: 0xffc85a}
			f := newPanelFixture(t, os, initial)
			w := f.window
			if len(f.opened) != 1 || f.opened[0].Kind != nsgotest.SurfacePanel || !f.opened[0].Transparent || !f.opened[0].NoActivate || f.opened[0].HideOnBlur {
				t.Fatalf("lyrics must open as a persistent, transparent, nonactivating panel: %+v", f.opened)
			}
			if f.background&0xff != 0 || f.drawing == nil {
				t.Fatalf("idle panel background=%#x, drawing=%v", f.background, f.drawing != nil)
			}
			if events := w.Poll(); len(events) != 0 {
				t.Fatalf("unreported frame was saved: %+v", events)
			}
			original := f.drawing
			initial.Words[0].Text = "mutated"
			if w.frame.Words[0].Text != "你好" {
				t.Fatal("panel retained caller's mutable words")
			}
			f.surface().Dispatch(nsgotest.Event{Kind: nsgotest.EventMouseEnter, ID: f.ids["desktop-lyrics-panel"]})
			f.flush(t)
			if f.background&0xff == 0 {
				t.Fatal("hover did not reveal the toolbar background")
			}
			for _, id := range []string{"previous", "play", "next", "lock", "close"} {
				node := f.ids["desktop-lyrics-"+id]
				if node == 0 {
					t.Fatalf("missing %s control on hover", id)
				}
				f.surface().Dispatch(nsgotest.Event{Kind: nsgotest.EventPress, ID: node})
			}
			var actions []Action
			for _, e := range w.Poll() {
				actions = append(actions, e.Action)
			}
			if !slices.Equal(actions, []Action{Previous, TogglePlay, Next, Lock, Hide}) {
				t.Fatalf("control actions: %v", actions)
			}
			updated := Frame{Words: []Word{{Text: "再见", Progress: .75}}, Secondary: "Goodbye", FontSize: 48, Color: 0x63e6ff, Locked: true}
			w.Update(updated)
			f.flush(t)
			if last := f.commands[len(f.commands)-1]; last.PointerEvents != desktop.WindowPointerNone {
				t.Fatalf("lock did not pass pointer through: %+v", last)
			}
			if f.background&0xff != 0 || f.drawing == original {
				t.Fatal("locked panel retained hover background or stale lyric drawing")
			}
			updated.Locked = false
			w.Update(updated)
			f.flush(t)
			if last := f.commands[len(f.commands)-1]; last.PointerEvents != desktop.WindowPointerAuto {
				t.Fatalf("external unlock failed: %+v", last)
			}
			if f.background&0xff != 0 {
				t.Fatal("unlock restored stale hover after click-through")
			}
			// Applying style changes keeps the same live panel.
			if w.panel.Closed() || !w.panel.Visible() || len(f.opened) != 1 {
				t.Fatal("style updates replaced or hid the panel")
			}
			w.Close()
			f.flush(t)
			if !w.panel.Closed() {
				t.Fatal("Close did not destroy the NSGo panel")
			}
			w.Update(updated)
			if len(w.Poll()) != 0 {
				t.Fatal("closed panel still emits events")
			}
		})
	}
}

func TestPanelRestoresLockAndReportsPositionInPoints(t *testing.T) {
	f := newPanelFixture(t, core.OSWindows, Frame{Locked: true, Positioned: true, LegacyPixels: true, X: 246, Y: 912, FontSize: 36})
	if f.opened[0].PointerEvents != desktop.WindowPointerNone {
		t.Fatal("restored panel briefly intercepts clicks")
	}
	var target desktop.WindowFrame
	for _, c := range f.commands {
		if c.Action == core.WindowSetFrame {
			target = c.Frame
		}
	}
	if target.X != 123 || target.Y != 456 {
		t.Fatalf("legacy pixel position not converted: %+v", target)
	}
	env := core.Environment{OS: core.OSWindows, WindowTransparent: true, WindowFrame: desktop.WindowFrame{Move: true, X: 321, Y: 456, Width: 800, Height: 154}}
	f.surface().SetEnvironment(env)
	events := f.window.Poll()
	if len(events) != 1 || !events[0].Moved || events[0].X != 321 || events[0].Y != 456 {
		t.Fatalf("position not reported in points: %+v", events)
	}
	if len(f.window.Poll()) != 0 {
		t.Fatal("same position repeatedly reported")
	}
	env.WindowFrame.X = 432
	f.surface().SetEnvironment(env)
	if events = f.window.Poll(); len(events) != 1 || events[0].X != 432 {
		t.Fatalf("drag did not report new position: %+v", events)
	}
	f.root.Dispose()
	if !f.window.panel.Closed() {
		t.Fatal("app scope cleanup leaked panel")
	}
}

func display(x, y, width, height core.Points, scale float32, main bool) core.DisplayInfo {
	b := core.Bounds{Origin: core.Point{X: x, Y: y}, Size: core.Size{Width: width, Height: height}}
	return core.DisplayInfo{Frame: b, VisibleFrame: b, Scale: scale, Main: main}
}

func TestPanelPositionMonitorChanges(t *testing.T) {
	tests := []struct {
		name        string
		frame       Frame
		displays    []core.DisplayInfo
		x, y, width core.Points
	}{
		{"default", Frame{}, []core.DisplayInfo{display(0, 0, 1440, 900, 2, true)}, 320, 706, 800},
		{"removed display", Frame{Positioned: true, X: 4000, Y: 2000}, []core.DisplayInfo{display(0, 0, 1440, 900, 2, true)}, 640, 746, 800},
		{"small display", Frame{}, []core.DisplayInfo{display(0, 0, 600, 400, 1, true)}, 0, 206, 600},
		{"negative origin", Frame{Positioned: true, X: -1100, Y: -200}, []core.DisplayInfo{display(0, 0, 1440, 900, 1, true), display(-1200, -300, 1200, 900, 2, false)}, -1100, -200, 800},
		{"mixed DPI legacy", Frame{Positioned: true, LegacyPixels: true, X: 2400, Y: 300}, []core.DisplayInfo{display(0, 0, 1920, 1080, 1, true), display(1920, 0, 1440, 900, 2, false)}, 2160, 150, 800},
		{"same DPI legacy", Frame{Positioned: true, LegacyPixels: true, X: -2200, Y: 400}, []core.DisplayInfo{display(0, 0, 1440, 900, 2, true), display(-1200, 0, 1200, 900, 2, false)}, -1100, 200, 800},
		{"points unchanged", Frame{Positioned: true, X: 300, Y: 400}, []core.DisplayInfo{display(0, 0, 1440, 900, 2, true)}, 300, 400, 800},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := initialFrame(tt.frame, tt.displays)
			if got.X != tt.x || got.Y != tt.y || got.Width != tt.width || !got.Move {
				t.Fatalf("got %+v; expected (%v,%v), width %v", got, tt.x, tt.y, tt.width)
			}
		})
	}
}

func TestLyricsDrawingClipsProgressAfterOutlines(t *testing.T) {
	var d core.Drawing
	words := []Word{{Text: "你", Progress: .5}, {Text: "好", Progress: 0}, {Text: "呀", Progress: 2}}
	drawLyrics(&d, core.Size{Width: 800, Height: 110}, Frame{Words: words, FontSize: 48, Color: 0x63e6ff, Secondary: "Hello"})
	var fills []string
	var clips []float32
	outlines, filled := 0, false
	for i, op := range d.Ops {
		if op.Kind == core.DrawStrokeText && op.Text != "Hello" {
			if filled {
				t.Fatal("a later outline covers a filled glyph")
			}
			outlines++
		}
		if op.Kind == core.DrawFillText {
			fills = append(fills, op.Text)
			filled = true
		}
		if op.Kind == core.DrawClip {
			clips = append(clips, d.Ops[i-1].W)
		}
	}
	if outlines != 3 || !slices.Equal(fills, []string{"你", "好", "呀", "你", "呀", "Hello"}) {
		t.Fatalf("outline/fill passes: %d, %v", outlines, fills)
	}
	font := core.Font{Size: 48, Weight: core.FontWeightSemibold}
	if len(clips) != 2 || math.Abs(float64(clips[0]-d.MeasureText("你", font)/2)) > .001 || clips[1] != d.MeasureText("呀", font) {
		t.Fatalf("partial and clamped clips: %v", clips)
	}
	var long core.Drawing
	drawLyrics(&long, core.Size{Width: 600, Height: 110}, Frame{Words: []Word{{Text: strings.Repeat("长句", 100)}}, FontSize: 48, Secondary: strings.Repeat("translation", 100)})
	scales := 0
	for _, op := range long.Ops {
		if op.Kind == core.DrawScale && op.X > 0 && op.X < 1 {
			scales++
		}
	}
	if scales != 2 {
		t.Fatal("main and secondary long lines must both fit panel width")
	}
}

func TestPanelUnavailableWithoutDesktopHost(t *testing.T) {
	for _, os := range []core.OS{core.OSWindows, core.OSAndroid} {
		t.Run(string(os), func(t *testing.T) {
			ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				if w, err := Open(cx, Frame{}); w != nil || err == nil {
					t.Fatal("opened without a desktop window host")
				}
				return core.View{}
			}, nsgotest.Environment(core.Environment{OS: os}))
			if len(ui.Opened) != 0 {
				t.Fatal("unsupported environment opened a surface")
			}
		})
	}
}
