package player

import (
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
)

func TestLocalCompanionsFollowPlaybackAndKeepAudio(t *testing.T) {
	p, out, ch := newNativeTest()
	defer p.Close()
	requested := make(chan music.Track, 4)
	release := make(chan struct{})
	p.CompleteLocal = func(tr music.Track) (music.Track, *music.Lyrics) {
		requested <- tr
		<-release
		tr.Album.ImageURL = "file:///missing-cover.png"
		return tr, &music.Lyrics{LRC: "[00:01.00]补全歌词"}
	}
	tr := music.Track{ID: "one", Source: "local", Duration: 60000}
	p.Restore(Session{Queue: []music.Track{tr}, Order: []int{0}, At: 0, Position: 12000})
	nativeUntil(t, ch, func() bool { return out.source != "" })
	out.events.Loaded(time.Minute)
	nativeUntil(t, ch, p.Ready)
	select {
	case <-requested:
		t.Fatal("paused restore requested network")
	default:
	}
	p.TogglePlay()
	select {
	case <-requested:
	case <-time.After(time.Second):
		t.Fatal("resume did not complete companions")
	}
	source, seek := out.source, out.seek
	close(release)
	nativeUntil(t, ch, func() bool { l, _ := p.Lyrics(); return len(l) > 0 })
	if out.source != source || out.seek != seek || p.Position() != 12*time.Second || p.Queue()[0].Album.ImageURL == "" {
		t.Fatal("completion restarted audio or lost queue artwork")
	}
	p.TogglePlay()
	p.TogglePlay()
	select {
	case <-requested:
		t.Fatal("completion repeated on pause/resume")
	default:
	}
}

func TestLocalCompanionLateResponseDoesNotChangeNextSong(t *testing.T) {
	p, out, ch := newNativeTest()
	defer p.Close()
	entered, release := make(chan struct{}), make(chan struct{})
	p.CompleteLocal = func(tr music.Track) (music.Track, *music.Lyrics) {
		close(entered)
		<-release
		tr.Album.ImageURL = "file:///wrong.png"
		return tr, &music.Lyrics{LRC: "[00:01.00]old"}
	}
	p.PlayTrack([]music.Track{{ID: "one", Source: "local", Duration: 60000}}, 0)
	nativeUntil(t, ch, func() bool { return out.source != "" })
	out.events.Loaded(time.Minute)
	nativeUntil(t, ch, p.Ready)
	<-entered
	p.PlayTrack([]music.Track{{ID: "two", Source: "other", Duration: 60000}}, 0)
	nativeUntil(t, ch, func() bool { return out.source == "file:///music/two.mp3" })
	out.events.Loaded(time.Minute)
	nativeUntil(t, ch, func() bool { _, loaded := p.Lyrics(); return p.Ready() && loaded })
	for len(ch) > 0 {
		(<-ch)()
	}
	close(release)
	select {
	case fn := <-ch:
		fn()
	case <-time.After(time.Second):
		t.Fatal("completion was not posted")
	}

	if p.Current().ID != "two" || p.Current().Album.ImageURL != "" {
		t.Fatal("late completion replaced the next song")
	}
	if l, _ := p.Lyrics(); len(l) > 0 {
		t.Fatal("late lyrics reached the next song")
	}
}
