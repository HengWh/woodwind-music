package app

import (
	"fmt"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"

	"nsgo-music/internal/i18n"
	"nsgo-music/internal/music"
)

// palette holds the app's colors, for the light and the dark appearance.
type palette struct {
	dark      bool
	bg        core.Color
	sidebar   core.Color
	text      core.Color
	muted     core.Color
	faint     core.Color
	hairline  core.Color
	hover     core.Color
	press     core.Color
	accent    core.Color
	onAccent  core.Color
	cardShade core.Color
	white     core.Color
}

var (
	lucideChevronLeft  = lucide.ChevronLeft()
	lucideChevronRight = lucide.ChevronRight()
	lucideChevronDown  = lucide.ChevronDown()
	lucideNote         = lucide.Music()
	lucideFolder       = lucide.FolderPlus()
	lucideSearch       = lucide.Search()
	lucideHeart        = lucide.Heart()
	lucideHome         = lucide.House()
	lucideList         = lucide.ListMusic()
	lucideDisc         = lucide.Disc3()
	lucideUser         = lucide.User()
	lucidePlay         = lucide.Play()
	lucidePause        = lucide.Pause()
	lucidePrev         = lucide.SkipBack()
	lucideNext         = lucide.SkipForward()
	lucideShuffle      = lucide.Shuffle()
	lucideRepeat       = lucide.Repeat()
	lucideRepeatOne    = lucide.Repeat1()
	lucideVolume       = lucide.Volume2()
	lucideVolumeMute   = lucide.VolumeX()
	lucideSettings     = lucide.Settings()
	lucidePlus         = lucide.Plus()
	lucideCheck        = lucide.Check()
	lucideMore         = lucide.Ellipsis()
	lucideX            = lucide.X()
	lucideTrash        = lucide.Trash()
	lucidePencil       = lucide.Pencil()
	lucideListPlus     = lucide.ListPlus()
	lucideMic          = lucide.AudioLines()
	lucideArrowUp      = lucide.ArrowUp()
	lucideArrowDown    = lucide.ArrowDown()
)

func (a *app) palette(cx *core.Scope) palette {
	if cx.Dark() {
		return palette{
			dark:      true,
			bg:        core.RGB(0x141416),
			sidebar:   core.RGB(0x1c1c1f),
			text:      core.RGB(0xf5f5f7),
			muted:     core.RGBA(0xebebf5, 0.56),
			faint:     core.RGBA(0xebebf5, 0.30),
			hairline:  core.RGBA(0xffffff, 0.07),
			hover:     core.RGBA(0xffffff, 0.06),
			press:     core.RGBA(0xffffff, 0.10),
			accent:    core.RGB(0xf45058),
			onAccent:  core.RGB(0xffffff),
			cardShade: core.RGBA(0x000000, 0.45),
			white:     core.RGB(0xffffff),
		}
	}
	return palette{
		bg:        core.RGB(0xffffff),
		sidebar:   core.RGB(0xf4f4f6),
		text:      core.RGB(0x1d1d1f),
		muted:     core.RGBA(0x3c3c43, 0.62),
		faint:     core.RGBA(0x3c3c43, 0.32),
		hairline:  core.RGBA(0x000000, 0.07),
		hover:     core.RGBA(0x000000, 0.04),
		press:     core.RGBA(0x000000, 0.08),
		accent:    core.RGB(0xe2333c),
		onAccent:  core.RGB(0xffffff),
		cardShade: core.RGBA(0x000000, 0.16),
		white:     core.RGB(0xffffff),
	}
}

// text is a run of the app's type.
func text(s string, size core.Points, weight core.FontWeight, color core.Color) core.Text {
	return core.Text{Content: s, Style: core.Style{FontSize: size, FontWeight: weight, Color: color}}
}

func textf(s string, size core.Points, weight core.FontWeight, color core.Color) core.Text {
	t := text(s, size, weight, color)
	return t
}

// icon is an icon in a color.
func icon(svg *core.SVG, size core.Points, color core.Color) core.Node {
	return core.Icon{SVG: svg, Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), Color: color}}
}

// roundButton is a borderless button showing an icon, which a soft circle
// marks under the pointer and which gives a little as it is pressed.
func (a *app) roundButton(cx *core.Scope, p palette, size core.Points, color core.Color, label string, svg *core.SVG, onPress func()) core.Node {
	return base.Button{Label: label, OnPress: func(core.PressEvent) {
		if onPress != nil {
			onPress()
		}
	}, StyleFunc: func() core.Style {
		return core.Style{Width: core.Pt(size + 16), Height: core.Pt(size + 16), Direction: core.Row, AlignItems: core.AlignCenter,
			JustifyContent: core.JustifyCenter, CornerRadius: 999, Color: color,
			Hover: &core.Style{Background: p.hover}, Active: &core.Style{Background: p.press}}
	}, Children: []core.Node{icon(svg, size, color)}}
}

// pillButton is a rounded button with an icon and a label.
func (a *app) pillButton(p palette, svg *core.SVG, label string, bg, fg core.Color, onPress func()) core.Node {
	var children []core.Node
	if svg != nil {
		children = append(children, icon(svg, 14, fg))
	}
	children = append(children, text(label, 14, core.FontWeightSemibold, fg))
	return base.Button{Label: label, OnPress: func(core.PressEvent) {
		if onPress != nil {
			onPress()
		}
	}, StyleFunc: func() core.Style {
		return core.Style{Height: core.Pt(36), Padding: core.Symmetric(0, 18), Gap: 8, Direction: core.Row, AlignItems: core.AlignCenter, AlignSelf: core.AlignStart,
			CornerRadius: 999, Background: bg, Color: fg, Hover: &core.Style{Background: blend(bg, core.RGB(0xffffff), 0.12)},
			Active: &core.Style{Background: blend(bg, core.RGB(0x000000), 0.12)}}
	}, Children: children}
}

func blend(a, b core.Color, t float32) core.Color {
	ar, ag, ab, aa := a.Components()
	br, bg, bb, ba := b.Components()
	m := func(x, y float64) uint32 { return uint32((x+(y-x)*float64(t))*255 + 0.5) }
	return core.RGBA(m(ar, br)<<16|m(ag, bg)<<8|m(ab, bb), float32(aa+(ba-aa)*float64(t)))
}

// cover shows an artwork, or a placeholder of the same size while it
// loads, or when there is none.
func (a *app) cover(p palette, url string, size core.Points, radius core.Points) core.Node {
	if url == "" {
		return core.View{Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), CornerRadius: radius,
			Background: p.sidebar, Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			Children: []core.Node{icon(lucideNote, size*0.32, p.faint)}}
	}
	return core.Image{Source: core.Src(url), ResizeMode: core.Cover, FadeDuration: 160 * time.Millisecond,
		Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), CornerRadius: radius, Background: p.sidebar}}
}

// clock says a length as m:ss, or h:mm:ss when it takes an hour.
func clock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	s := int(d.Round(time.Second) / time.Second)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// trackRow is a row of a track list: its number, cover, title, artist, album
// and length.
func (a *app) trackRow(p palette, tracks []music.Track, i int, showCover bool, showAlbum bool, playing bool, _ bool) core.Node {
	tr := tracks[i]
	isCurrent := func() bool {
		if t := a.pl.Current(); t != nil {
			return t.ID == tr.ID
		}
		return false
	}
	row := base.Button{Label: tr.Name, TestID: "track-" + tr.ID, OnPress: func(core.PressEvent) { a.playTracks(tracks, i) },
		StyleFunc: func() core.Style {
			s := core.Style{Height: core.Pt(54), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 14,
				Padding: core.Symmetric(4, 12), CornerRadius: 10, Color: p.text,
				Hover: &core.Style{Background: p.sidebar}}
			a.playRev.Get()
			if isCurrent() {
				s.Background = p.hover
			}
			return s
		}}
	// The number says where it is in the list, or the bars of the song
	// playing, as a song's row does in Music.
	children := []core.Node{core.View{Style: core.Style{Width: core.Pt(26), Direction: core.Row, AlignItems: core.AlignCenter,
		JustifyContent: core.JustifyCenter}, Children: []core.Node{core.Show{When: func() bool { a.playRev.Get(); return isCurrent() },
		Then: func(*core.Scope) core.Node { return icon(lucideVolume, 14, p.accent) },
		Else: func(*core.Scope) core.Node { return text(fmt.Sprint(i+1), 13, core.FontWeightNormal, p.faint) }}}}}
	if showCover {
		children = append(children, core.View{Style: core.Style{Width: core.Pt(40), FlexShrink: 0}, Children: []core.Node{a.cover(p, tr.Album.ImageURL, 40, 6)}})
	}
	children = append(children, core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Direction: core.Column, Gap: 3}, Children: []core.Node{
		core.Text{Content: tr.Name, MaxLines: 1, StyleFunc: func() core.Style {
			a.playRev.Get()
			c := p.text
			if isCurrent() {
				c = p.accent
			}
			return core.Style{FontSize: 13.5, FontWeight: core.FontWeightMedium, Color: c}
		}},
		a.artistLine(p, tr, 12, p.muted),
	}})
	if showAlbum {
		children = append(children, core.Text{Content: tr.Album.Name, MaxLines: 1, Style: core.Style{Width: core.Pct(26), FontSize: 12.5, Color: p.muted}})
	}
	children = append(children, a.likeButton(p, tr.ID, 15, p.muted))
	children = append(children, core.Text{Content: clock(tr.Length()), Style: core.Style{Width: core.Pt(44), TextAlign: core.TextAlignEnd, FontSize: 12, Color: p.faint}})
	row.Children = children
	return row
}

// artistLine names a track's artists, each a link to its page.
func (a *app) artistLine(p palette, tr music.Track, size core.Points, color core.Color) core.Text {
	spans := make([]core.Span, 0, max(0, len(tr.Artists)*2-1))
	for i, ar := range tr.Artists {
		if i > 0 {
			spans = append(spans, core.Span{Content: ", "})
		}
		if ar.ID == "" {
			spans = append(spans, core.Span{Content: ar.Name})
			continue
		}
		id := ar.ID
		spans = append(spans, core.Span{Content: ar.Name, Style: core.Style{Color: color},
			OnPress: func(core.PressEvent) { a.open(route{page: pageArtist, id: id}) }})
	}
	if len(spans) == 0 {
		return text("", size, core.FontWeightNormal, color)
	}
	return core.Text{Spans: spans, MaxLines: 1, Style: core.Style{FontSize: size, Color: color}}
}

// likeButton is the heart of a track.
func (a *app) likeButton(p palette, id string, size core.Points, color core.Color) core.Node {
	liked := a.likedSet[id]
	fill := p.accent
	if liked {
		color = fill
	}
	return base.Button{Label: i18n.T("Like"), OnPress: func(core.PressEvent) { a.toggleLike(id) },
		StyleFunc: func() core.Style {
			return core.Style{Width: core.Pt(size + 14), Height: core.Pt(size + 14), Direction: core.Row, AlignItems: core.AlignCenter,
				JustifyContent: core.JustifyCenter, CornerRadius: size / 2, Hover: &core.Style{Background: p.sidebar}}
		}, Children: []core.Node{icon(lucideHeart, size, color)}}
}

// sectionTitle is the heading of a row of cards.
func (a *app) sectionTitle(p palette, title string) core.Node {
	return core.Text{Content: title, Style: core.Style{FontSize: 19, FontWeight: core.FontWeightBold, LetterSpacing: -0.3,
		Color: p.text, Margin: core.Edges{Top: 34, Bottom: 14}}}
}

// sectionHeader is a section's title, with See All at its end when the
// section has a page of its own.
func (a *app) sectionHeader(p palette, title, more string) core.Node {
	var children []core.Node
	if title != "" {
		children = append(children, core.Text{Content: title, Style: core.Style{FontSize: 19, FontWeight: core.FontWeightBold,
			LetterSpacing: -0.3, Color: p.text, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}})
	} else {
		children = append(children, core.View{Style: core.Style{FlexGrow: 1}})
	}
	if more != "" {
		page := more
		children = append(children, base.Button{Label: i18n.T("See All"), OnPress: func(core.PressEvent) {
			a.open(route{page: pageBrowse, id: page})
		}, Style: core.Style{Height: core.Pt(26), Padding: core.Symmetric(0, 8), Direction: core.Row, AlignItems: core.AlignCenter,
			CornerRadius: 6, Hover: &core.Style{Background: p.hover}},
			Children: []core.Node{text(i18n.T("See All"), 13, core.FontWeightNormal, p.muted)}})
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Margin: core.Edges{Top: 34, Bottom: 14}},
		Children: children}
}

// pageTitle is the heading of a page.
func (a *app) pageTitle(p palette, title, sub string) core.Node {
	var children []core.Node
	if sub != "" {
		children = append(children, text(sub, 12, core.FontWeightSemibold, p.muted))
	}
	children = append(children, text(title, 30, core.FontWeightBold, p.text))
	return core.View{Style: core.Style{Gap: 4, Margin: core.Edges{Bottom: 18}}, Children: []core.Node{core.Group(children)}}
}

// emptyNote is a page with nothing to show.
func (a *app) emptyNote(p palette, big, small string) core.Node {
	return core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Padding: core.All(40), Gap: 8},
		Children: []core.Node{
			text(big, 22, core.FontWeightBold, p.text),
			text(small, 13.5, core.FontWeightNormal, p.muted),
		}}
}

// grid returns how many cards of at least min points fit across width.
func columns(width core.Points, min core.Points) int {
	return max(2, int((width+20)/(min+20)))
}
