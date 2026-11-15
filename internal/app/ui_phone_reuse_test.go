package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// Native view identity matters: virtualizing a list's cell is not enough if
// every recycled song destroys all of the views inside that cell.
func TestPhoneSongRowsKeepViewsWhileScrolling(t *testing.T) {
	ui, a := startPhone(t)
	tracks := make([]music.Track, 300)
	for i := range tracks {
		tracks[i] = music.Track{ID: fmt.Sprintf("reuse-%d", i), Name: fmt.Sprintf("Reuse song %03d", i), Artists: []music.Artist{{Name: fmt.Sprintf("Artist %03d", i)}}}
	}
	a.post(func() {
		a.reloadGen++
		a.browses["reuse"] = &browseData{browse: &music.Browse{Title: "Reuse", Sections: []music.Section{{Tracks: tracks}}}}
		a.route.Set(route{page: pageBrowse, id: "reuse"})
		a.bump()
	})
	ui.Settle()
	ui.Advance(time.Second)
	ids := map[string]bool{}
	collect := func() {
		var walk func(*nsgotest.View)
		walk = func(v *nsgotest.View) {
			if strings.HasPrefix(v.Props.TestID, "phone-row-") {
				ids[fmt.Sprint(v.ID)] = true
			}
			for _, child := range v.Children {
				walk(child)
			}
		}
		walk(ui.FindMounted("id:app-root"))
	}
	collect()
	scroll := ui.Find("id:phone-page-scroll")
	end := scroll.Content.Height - scroll.Frame.Size.Height
	for y := core.Points(400); y < end; y += 400 {
		ui.ScrollTo("id:phone-page-scroll", core.Point{Y: y})
		collect()
	}
	scroll = ui.Find("id:phone-page-scroll")
	ui.ScrollTo("id:phone-page-scroll", core.Point{Y: scroll.Content.Height - scroll.Frame.Size.Height})
	collect()
	t.Logf("distinct native song rows across 300 songs: %d", len(ids))
	if len(ids) == 0 || len(ids) > 64 {
		t.Fatalf("scrolling rebuilt %d song row views instead of retaining a bounded pool", len(ids))
	}
	ui.Find(`label:"Reuse song 299"`)
	ui.Press(fmt.Sprintf("label:%q", i18n.Tf("More: %s", "Reuse song 299")))
	if a.phoneTrack.ID != "reuse-299" {
		t.Fatalf("recycled action opened %q", a.phoneTrack.ID)
	}
}
