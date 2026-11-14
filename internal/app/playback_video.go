package app

import (
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/core"

	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/player"
)

// videoOutput keeps the platform player mounted independently of navigation,
// lyrics, settings and theme redraws. Only opening a track recreates the node.
type videoOutput struct {
	request *core.Signal[*videoRequest]
	paused  *core.Signal[bool]
	volume  *core.Signal[float32]
	muted   *core.Signal[bool]
	relay   *music.StreamRelay
	managed bool
}

type videoRequest struct {
	source string
	events player.NativeEvents
	ref    core.Ref
}

func newVideoOutput(cx *core.Scope) *videoOutput {
	return &videoOutput{
		request: core.NewSignal[*videoRequest](cx, nil),
		paused:  core.NewSignal(cx, true),
		volume:  core.NewSignal[float32](cx, 1),
		muted:   core.NewSignal(cx, false),
		managed: cx.HostParams()["woodwind.android"] == true,
	}
}

func (v *videoOutput) Open(source string, paused bool, events player.NativeEvents) {
	if v.relay != nil {
		v.relay.Close()
		v.relay = nil
	}
	v.paused.Set(paused)
	v.request.Set(&videoRequest{source: source, events: events})
}

func (v *videoOutput) OpenWithHeaders(source string, headers map[string]string, paused bool, events player.NativeEvents) {
	v.Close()
	if len(headers) == 0 && !strings.HasPrefix(strings.ToLower(source), "http://") {
		v.Open(source, paused, events)
		return
	}
	relay, err := music.NewStreamRelay(source, headers)
	if err != nil {
		events.Error(err)
		return
	}
	v.Open(relay.URL, paused, events)
	v.relay = relay
}

func (v *videoOutput) SetPaused(paused bool) { v.paused.Set(paused) }

func (v *videoOutput) SetVolume(volume float64) {
	volume = max(0, min(1, volume))
	// core.Video treats a literal zero volume as its default full volume.
	// Muted must therefore accompany zero, including changes while loading.
	v.muted.Set(volume == 0)
	v.volume.Set(float32(volume))
}

func (v *videoOutput) Seek(position time.Duration) {
	if r := v.request.GetUntracked(); r != nil {
		r.ref.Seek(position)
	}
}

func (v *videoOutput) Close() {
	v.request.Set(nil)
	if v.relay != nil {
		v.relay.Close()
		v.relay = nil
	}
}

func (v *videoOutput) view() core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		r := v.request.Get()
		if r == nil {
			return core.Group{}
		}
		var session core.AudioSession
		if !v.managed {
			session = core.AudioSessionPlayback
		}
		return core.Video{
			TestID: "android-audio", Source: r.source, Ref: &r.ref,
			PausedFunc: v.paused.Get, VolumeFunc: v.volume.Get, MutedFunc: v.muted.Get,
			AudioSession: session, ProgressInterval: 200 * time.Millisecond,
			// Keep a real node mounted for audio-only playback; it occupies
			// no layout space and never becomes a visible video surface.
			Style:      core.Style{Position: core.Absolute, Width: core.Pt(1), Height: core.Pt(1), Opacity: core.Alpha(0)},
			OnLoad:     func(info core.VideoInfo) { r.events.Loaded(info.Duration) },
			OnProgress: r.events.Progress, OnBuffer: r.events.Buffer,
			OnEnd: r.events.Ended, OnError: r.events.Error,
		}
	})
}
