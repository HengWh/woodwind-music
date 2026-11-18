package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/offline"
)

type storedUI struct {
	query                                              *core.Signal[string]
	mode, source, quality, complete, sort, view, group string
	selected                                           map[string]bool
	tasks                                              bool
	details                                            string
	deleteIDs                                          []string
}

func (a *app) openStored() { a.open(route{page: pageStored}) }
func (a *app) storedWork(run func() error) {
	go func() {
		err := run()
		a.post(func() {
			if err != nil {
				a.say(err.Error())
			}
			a.bump()
		})
	}()
}
func (a *app) pinTracks(tracks []music.Track, q string) {
	if a.disk == nil {
		return
	}
	copies := slices.Clone(tracks)
	source := a.source().Info().ID
	quality := q
	defaultQuality := a.set.Quality
	for i := range copies {
		if copies[i].Source == "" && !copies[i].LocalOnly {
			copies[i].Source = source
		}
	}
	a.storedWork(func() error {
		var failures []error
		for _, t := range copies {
			target := quality
			if target == "" && !a.disk.Has(t) {
				target = defaultQuality
			}
			if err := a.disk.Pin(t, target, false); err != nil {
				failures = append(failures, err)
			}
		}
		if len(failures) == 0 {
			a.post(func() { a.say(i18n.T("Offline requests added. View progress in Saved Music.")) })
		}
		return errors.Join(failures...)
	})
}
func (a *app) storedTracks(records []offline.Record) []music.Track {
	tracks := make([]music.Track, len(records))
	for i, r := range records {
		tracks[i] = a.disk.Track(r)
	}
	return tracks
}
func (a *app) selectedStored(records []offline.Record) []offline.Record {
	var out []offline.Record
	for _, r := range records {
		if a.diskUI.selected[r.ID] {
			out = append(out, r)
		}
	}
	return out
}
func (a *app) storedFilter(records []offline.Record) []offline.Record {
	u := &a.diskUI
	q := ""
	if u.query != nil {
		q = strings.ToLower(strings.TrimSpace(u.query.Get()))
	}
	var out []offline.Record
	for _, r := range records {
		if u.mode == "pinned" && !r.Pinned() || u.mode == "automatic" && r.Pinned() {
			continue
		}
		if u.source != "" && r.Track.Source != u.source {
			continue
		}
		if u.quality != "" && !slices.ContainsFunc(r.Variants, func(v offline.Variant) bool { return v.Stream.Quality == u.quality }) {
			continue
		}
		full := r.Cover != "" && r.Lyrics != nil
		if u.complete == "complete" && !full || u.complete == "missing" && full {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(r.Track.Name+" "+r.Track.ArtistNames()+" "+r.Track.Album.Name), q) {
			continue
		}
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b offline.Record) int {
		switch u.sort {
		case "title":
			return strings.Compare(strings.ToLower(a.Track.Name), strings.ToLower(b.Track.Name))
		case "artist":
			return strings.Compare(a.Track.ArtistNames(), b.Track.ArtistNames())
		case "size":
			if a.Size() > b.Size() {
				return -1
			}
			if a.Size() < b.Size() {
				return 1
			}
		case "played":
			if a.Played > b.Played {
				return -1
			}
			if a.Played < b.Played {
				return 1
			}
		default:
			if a.Created > b.Created {
				return -1
			}
			if a.Created < b.Created {
				return 1
			}
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}
func (a *app) storedChoice(p palette, id, label string, value *string, options []ui.Option[string]) core.Node {
	return settingsChoice[string]{palette: p, Touch: a.phoneLayout(a.cx), TestID: id, Label: label, Options: options, Value: func() string { return *value }, OnChange: func(v string) { *value = v; a.diskUI.group = ""; a.bump() }, Style: core.Style{Width: core.Pt(148)}}
}
func (a *app) storedPage(cx *core.Scope, p palette) core.Node {
	if a.disk == nil {
		return a.emptyNote(p, i18n.T("Saved Music"), a.diskErr)
	}
	if a.diskUI.query == nil {
		a.diskUI.query = core.NewSignal(a.cx, "")
	}
	if a.diskUI.selected == nil {
		a.diskUI.selected = map[string]bool{}
	}
	return storedScreen{a: a, p: p}
}

type storedScreen struct {
	a *app
	p palette
}

func (s storedScreen) Render(cx *core.Scope) core.Node {
	a, p := s.a, s.p
	return core.View{TestID: "stored-page", Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0), Direction: core.Column}, Children: []core.Node{
		core.View{Style: core.Style{Padding: core.Symmetric(8, 16)}, Children: []core.Node{
			base.Input{TestID: "stored-search", Label: i18n.T("Search saved music"), Placeholder: i18n.T("Search saved music"), Value: a.diskUI.query.Get, OnChange: func(v string) { a.diskUI.query.Set(v) }, Style: core.Style{Height: core.Pt(44), Padding: core.Symmetric(0, 12), CornerRadius: 10, Background: p.sidebar, Color: p.text, PlaceholderColor: p.muted}},
		}},
		core.Dynamic(func(cx *core.Scope) core.Node {
			a.rev.Get()
			a.diskRev.Get()
			a.diskUI.query.Get()
			return a.storedBody(cx, p)
		}),
	}}
}
func (a *app) storedBody(cx *core.Scope, p palette) core.Node {
	u := &a.diskUI
	all := a.disk.List()
	records := a.storedFilter(all)
	var body []core.Node
	body = append(body, a.titleBlock(p, i18n.T("Saved Music"), i18n.Tf("%d songs available on this device", len(all))))
	var tabs []core.Node
	for _, it := range []struct{ value, label string }{{"", i18n.T("All")}, {"pinned", i18n.T("Keep Offline")}, {"automatic", i18n.T("Automatic Cache")}} {
		it := it
		label := it.label
		if u.mode == it.value && !u.tasks {
			label = "✓ " + label
		}
		tabs = append(tabs, a.settingsAction(p, label, "stored-tab-"+it.value, false, func() { u.mode = it.value; u.tasks = false; u.group = ""; a.bump() }))
	}
	tabs = append(tabs, a.settingsAction(p, i18n.T("Download Tasks"), "stored-tasks", false, func() { u.tasks = !u.tasks; a.bump() }))
	body = append(body, storedBar(tabs))
	if u.tasks {
		return a.pageScroll(p, append(body, a.storedTasks(p))...)
	}
	sources := []ui.Option[string]{{Value: "", Label: i18n.T("All Sources")}}
	seen := map[string]bool{}
	for _, r := range all {
		if !seen[r.Track.Source] {
			sources = append(sources, ui.Option[string]{Value: r.Track.Source, Label: r.SourceName})
			seen[r.Track.Source] = true
		}
	}
	filters := []core.Node{
		a.storedChoice(p, "stored-source", i18n.T("Music source"), &u.source, sources),
		a.storedChoice(p, "stored-quality", i18n.T("Quality"), &u.quality, []ui.Option[string]{{Value: "", Label: i18n.T("All Qualities")}, {Value: "standard", Label: i18n.T("Standard")}, {Value: "high", Label: i18n.T("High")}, {Value: "lossless", Label: i18n.T("Lossless")}}),
		a.storedChoice(p, "stored-complete", i18n.T("Companion Files"), &u.complete, []ui.Option[string]{{Value: "", Label: i18n.T("All Details")}, {Value: "complete", Label: i18n.T("Complete Details")}, {Value: "missing", Label: i18n.T("Missing Details")}}),
		a.storedChoice(p, "stored-sort", i18n.T("Sort"), &u.sort, []ui.Option[string]{{Value: "", Label: i18n.T("Recently Saved")}, {Value: "played", Label: i18n.T("Recently Played")}, {Value: "title", Label: i18n.T("Name")}, {Value: "artist", Label: i18n.T("Artist")}, {Value: "size", Label: i18n.T("Storage Used")}}),
		a.storedChoice(p, "stored-view", i18n.T("View"), &u.view, []ui.Option[string]{{Value: "", Label: i18n.T("Songs")}, {Value: "albums", Label: i18n.T("Albums")}, {Value: "artists", Label: i18n.T("Artists")}}),
	}
	body = append(body, storedBar(filters))
	if u.view != "" {
		var groups []storedGroup
		groups = storedGroups(records, u.view)
		if u.group == "" {
			for _, g := range groups {
				g := g
				body = append(body, a.phoneLibraryRow(p, "stored-group-"+g.key, g.name, i18n.Tf("%d songs available on this device", len(g.records)), lucideDisc, func() { u.group = g.key; a.bump() }))
			}
			if len(groups) == 0 {
				body = append(body, text(i18n.T("No saved songs match"), 14, core.FontWeightNormal, p.muted))
			}
			return a.pageScroll(p, body...)
		}
		var chosen *storedGroup
		for _, g := range groups {
			if g.key == u.group {
				copy := g
				chosen = &copy
				break
			}
		}
		if chosen != nil {
			records = chosen.records
			body = append(body, text(chosen.name, 20, core.FontWeightSemibold, p.text))
		} else {
			records = nil
		}
		body = append(body, a.settingsAction(p, i18n.T("Back"), "stored-group-back", false, func() { u.group = ""; a.bump() }))
	}
	tracks := a.storedTracks(records)
	selected := a.selectedStored(records)
	targets := selected
	if len(targets) == 0 {
		targets = records
	}
	targetTracks := a.storedTracks(targets)
	ids := make([]string, len(targets))
	for i, r := range targets {
		ids[i] = r.ID
	}
	actions := []core.Node{
		a.settingsAction(p, i18n.T("Play All"), "stored-play-all", len(records) == 0, func() { a.playTracks(tracks, 0) }),
		a.settingsAction(p, i18n.T("Shuffle"), "stored-shuffle", len(records) == 0, func() { a.pl.PlayShuffled(tracks); a.heartbeat() }),
		a.settingsAction(p, i18n.T("Add to Queue"), "stored-enqueue", len(targets) == 0, func() { a.pl.Enqueue(targetTracks); a.heartbeat() }),
		a.settingsAction(p, i18n.T("Keep Offline"), "stored-pin", len(targets) == 0, func() { a.pinTracks(targetTracks, "") }),
		a.settingsAction(p, i18n.T("Cancel Offline Retention"), "stored-unpin", len(targets) == 0, func() { a.cancelStoredRetention(ids) }),
		a.settingsAction(p, i18n.T("Select All"), "stored-select-all", len(records) == 0, func() {
			for _, r := range records {
				u.selected[r.ID] = true
			}
			a.bump()
		}),
		a.settingsAction(p, i18n.T("Clear Selection"), "stored-select-none", len(selected) == 0, func() { u.selected = map[string]bool{}; a.bump() }),
		a.settingsAction(p, i18n.T("Remove Local Copies"), "stored-delete", len(selected) == 0, func() { u.deleteIDs = slices.Clone(ids); a.bump() }),
	}
	body = append(body, storedBar(actions))
	if len(selected) > 0 {
		body = append(body, text(i18n.Tf("%d selected", len(selected)), 12, core.FontWeightNormal, p.muted))
	}
	if len(records) == 0 {
		body = append(body, text(i18n.T("No saved songs match"), 14, core.FontWeightNormal, p.muted))
	}
	if u.details != "" {
		for _, r := range all {
			if r.ID == u.details {
				body = append(body, a.storedInfo(p, r))
			}
		}
	}
	body = append(body, core.List{Count: func() int { return len(records) }, Key: func(i int) any { return records[i].ID }, EstimatedSize: 76, Viewport: core.ListViewportParent, Row: func(cx *core.Scope, row core.ListRow) core.Node { return a.storedRow(p, records, tracks, row.Index()) }})
	return a.pageScroll(p, body...)
}
func storedBar(children []core.Node) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 8, AlignItems: core.AlignCenter, Margin: core.Edges{Bottom: 12}}, Children: children}
}

type storedGroup struct {
	key, name string
	records   []offline.Record
}

func storedGroups(records []offline.Record, view string) []storedGroup {
	groups := map[string]*storedGroup{}
	add := func(r offline.Record, id, name string) {
		if name == "" {
			name = i18n.T("Unknown")
		}
		key := offline.Key(r.Track.Source, id)
		g := groups[key]
		if g == nil {
			g = &storedGroup{key: key, name: name}
			groups[key] = g
		}
		g.records = append(g.records, r)
	}
	for _, r := range records {
		if view == "albums" {
			id := r.Track.Album.ID
			if id == "" {
				id = r.Track.Album.Name + "/" + r.Track.ArtistNames()
			}
			add(r, "album:"+id, r.Track.Album.Name)
		} else {
			if len(r.Track.Artists) == 0 {
				add(r, "artist:unknown", "")
			}
			for _, artist := range r.Track.Artists {
				id := artist.ID
				if id == "" {
					id = artist.Name
				}
				add(r, "artist:"+id, artist.Name)
			}
		}
	}
	var out []storedGroup
	for _, g := range groups {
		out = append(out, *g)
	}
	slices.SortFunc(out, func(a, b storedGroup) int { return strings.Compare(a.name, b.name) })
	return out
}
func savedName(r offline.Record) string {
	if r.Legacy {
		return i18n.T("Unidentified cached song")
	}
	return r.Track.Name
}
func bytesLabel(n int64) string {
	if n >= 1<<30 {
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
}
func variantLabel(v offline.Variant) string {
	quality := i18n.T("Unknown Quality")
	switch v.Stream.Quality {
	case "standard":
		quality = i18n.T("Standard")
	case "high":
		quality = i18n.T("High")
	case "lossless":
		quality = i18n.T("Lossless")
	}
	return strings.ToUpper(v.Stream.Type) + " · " + quality
}
func (a *app) storedRow(p palette, records []offline.Record, tracks []music.Track, index int) core.Node {
	r, tr := records[index], tracks[index]
	retention := i18n.T("Automatic Cache")
	if r.Pinned() {
		retention = i18n.T("Keep Offline")
	}
	label := r.Track.ArtistNames() + " · " + retention
	if len(r.Variants) > 0 {
		label += " · " + variantLabel(r.Variants[0])
	}
	label += " · " + bytesLabel(r.Size())
	selection := "○"
	if a.diskUI.selected[r.ID] {
		selection = "✓"
	}
	return core.View{TestID: "stored-row-" + r.ID, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(76), Borders: core.Edges{Bottom: 0.5}, BorderColor: p.hairline}, Children: []core.Node{
		a.settingsAction(p, selection, "stored-select-"+r.ID, false, func() { a.diskUI.selected[r.ID] = !a.diskUI.selected[r.ID]; a.bump() }),
		base.Button{TestID: "stored-play-" + r.ID, Label: savedName(r), OnPress: func(core.PressEvent) { a.playTracks(tracks, index) }, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Padding: core.Symmetric(8, 0)}, Children: []core.Node{a.cover(p, tr.Album.ImageURL, 48, 8), core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 5}, Children: []core.Node{core.Text{Content: savedName(r), MaxLines: 1, Style: core.Style{FontSize: 14, Color: p.text}}, core.Text{Content: label, MaxLines: 2, Style: core.Style{FontSize: 11.5, Color: p.muted}}}}}},
		base.Menu{TestID: "stored-more-" + r.ID, Label: i18n.T("More"), Trigger: text("•••", 18, core.FontWeightSemibold, p.muted), Style: core.Style{Padding: core.All(12)}, PopupStyle: core.Style{Background: p.sidebar, Padding: core.All(6), CornerRadius: 10}, Children: a.storedMenu(tr)},
	}}
}
func (a *app) storedMenu(tr music.Track) []core.Node {
	if a.disk == nil {
		return nil
	}
	var items []core.Node
	r, exists := a.disk.Get(tr)
	if tr.Source != "local" && (tr.Source != "" || exists) {
		items = append(items, ui.MenuItem{Text: i18n.T("Keep Offline"), OnSelect: func() { a.pinTracks([]music.Track{tr}, "") }})
	}
	if exists && len(r.Variants) > 0 {
		items = append(items, ui.MenuItem{Text: i18n.T("Add to Queue"), OnSelect: func() { a.pl.Enqueue([]music.Track{a.disk.Track(r)}); a.heartbeat() }}, ui.MenuItem{Text: i18n.T("Song Info"), OnSelect: func() { a.diskUI.details = r.ID; a.openStored() }})
		if r.Pinned() {
			items = append(items, ui.MenuItem{Text: i18n.T("Cancel Offline Retention"), OnSelect: func() { a.cancelStoredRetention([]string{r.ID}) }})
		}
		if src := a.sourceByID(r.Track.Source); src != nil {
			items = append(items, ui.MenuItem{Text: i18n.T("Keep in Lossless Quality"), OnSelect: func() { a.pinTracks([]music.Track{r.Track}, "lossless") }}, ui.MenuItem{Text: i18n.T("Fetch Missing Details"), OnSelect: func() {
				t := r.Track
				t.LocalOnly = false
				if original := a.pl.Current(); original != nil && original.ID == t.ID && original.Source == t.Source {
					t.Album.ImageURL = original.Album.ImageURL
				}
				go func() { <-a.disk.Fill(t, src, true); a.post(a.bump) }()
			}})
		}
		items = append(items, ui.MenuItem{Text: i18n.T("Remove Local Copies"), Variant: ui.MenuItemDanger, OnSelect: func() { a.diskUI.deleteIDs = []string{r.ID}; a.bump() }})
	}
	return items
}
func (a *app) storedInfo(p palette, r offline.Record) core.Node {
	source := r.SourceName
	if r.Legacy {
		source = i18n.T("Unknown source")
	} else if a.sourceByID(r.Track.Source) == nil {
		source += " · " + i18n.T("Plugin Removed")
	}
	info := savedName(r) + "\n" + r.Track.ArtistNames() + " · " + r.Track.Album.Name + "\n" + source
	for _, v := range r.Variants {
		info += "\n" + variantLabel(v) + " · " + bytesLabel(v.Stream.Size)
	}
	if r.Cover == "" {
		info += "\n" + i18n.T("Missing Cover")
	}
	if r.Lyrics == nil {
		info += "\n" + i18n.T("Missing Lyrics")
	}
	return core.View{TestID: "stored-info", Style: core.Style{Padding: core.All(14), Background: p.sidebar, CornerRadius: 10, Gap: 8, Margin: core.Edges{Bottom: 12}}, Children: []core.Node{text(info, 13, core.FontWeightNormal, p.text), a.settingsAction(p, i18n.T("Close"), "stored-info-close", false, func() { a.diskUI.details = ""; a.bump() })}}
}
func taskLabel(state string) string {
	switch state {
	case "queued":
		return i18n.T("Queued")
	case "resolving":
		return i18n.T("Finding Audio")
	case "downloading":
		return i18n.T("Downloading")
	case "saved":
		return i18n.T("Saved Offline")
	case "waiting-plugin":
		return i18n.T("Waiting for Plugin")
	case "waiting-login":
		return i18n.T("Waiting for Sign In")
	case "waiting-network":
		return i18n.T("Waiting for Wi-Fi")
	case "waiting-space":
		return i18n.T("Waiting for Storage Space")
	case "interrupted":
		return i18n.T("Interrupted")
	case "paused":
		return i18n.T("Paused")
	default:
		return i18n.T("Download Failed")
	}
}
func (a *app) storedTasks(p palette) core.Node {
	var children []core.Node
	tasks := a.disk.Tasks()
	if len(tasks) == 0 {
		children = append(children, text(i18n.T("No download tasks"), 14, core.FontWeightNormal, p.muted))
	}
	for _, task := range tasks {
		t := task
		sub := taskLabel(t.State) + " · " + bytesLabel(t.Received)
		if t.Total > 0 {
			sub += " / " + bytesLabel(t.Total)
		}
		if t.Error != "" {
			sub += "\n" + i18n.T(t.Error)
		}
		actions := []core.Node{}
		if t.State == "queued" || t.State == "resolving" || t.State == "downloading" {
			actions = append(actions, a.settingsAction(p, i18n.T("Pause"), "stored-task-pause-"+t.ID, false, func() { a.storedWork(func() error { a.disk.Pause(t.ID); return nil }) }))
		} else if t.State != "saved" {
			actions = append(actions, a.settingsAction(p, i18n.T("Retry"), "stored-task-retry-"+t.ID, false, func() { a.storedWork(func() error { return a.disk.Retry(t.ID, false) }) }))
			if t.State == "waiting-network" {
				actions = append(actions, a.settingsAction(p, i18n.T("Allow Mobile Data"), "stored-task-mobile-"+t.ID, false, func() { a.storedWork(func() error { return a.disk.Retry(t.ID, true) }) }))
			}
		}
		actions = append(actions, a.settingsAction(p, i18n.T("Remove"), "stored-task-remove-"+t.ID, false, func() { a.storedWork(func() error { a.disk.Cancel(t.ID); return nil }) }))
		children = append(children, core.View{TestID: "stored-task-" + t.ID, Style: core.Style{Padding: core.All(12), Gap: 8, Borders: core.Edges{Bottom: 1}, BorderColor: p.hairline}, Children: []core.Node{text(t.Track.Name, 14, core.FontWeightMedium, p.text), text(sub, 12, core.FontWeightNormal, p.muted), storedBar(actions)}})
	}
	return core.View{Style: core.Style{Direction: core.Column}, Children: children}
}
func (a *app) storedDeleteDialog(cx *core.Scope, p palette) core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		a.rev.Get()
		dialog := ui.Dialog{Open: func() bool { return len(a.diskUI.deleteIDs) > 0 }, OnOpenChange: func(open bool, _ base.Reason) {
			if !open {
				a.diskUI.deleteIDs = nil
				a.bump()
			}
		}, Title: i18n.T("Remove Local Copies"), Description: i18n.T("Remove these audio files and their saved details from this device? Offline copies are included. Online songs and playlists are unchanged."), Footer: []core.Node{ui.Button{Text: i18n.T("Cancel"), OnPress: func() { a.diskUI.deleteIDs = nil; a.bump() }}, ui.Button{Text: i18n.T("Remove"), TestID: "stored-delete-confirm", OnPress: func() {
			ids := slices.Clone(a.diskUI.deleteIDs)
			a.diskUI.deleteIDs = nil
			a.storedWork(func() error {
				for _, id := range ids {
					if err := a.disk.Delete(id); err != nil {
						return err
					}
				}
				return nil
			})
		}}}}
		return a.fitPhoneDialog(cx, p, dialog)
	})
}

func (a *app) cancelStoredRetention(ids []string) {
	copies := slices.Clone(ids)
	a.storedWork(func() error {
		for _, id := range copies {
			if err := a.disk.SetPinned(id, false); err != nil {
				return err
			}
		}
		a.post(func() { a.say(i18n.T("Offline retention canceled. These copies can now be automatically removed.")) })
		return nil
	})
}
