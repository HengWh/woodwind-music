package app

import (
	"math"

	"github.com/nsgo-dev/nsgo/core"
)

// Mobile sizes are logical points. NSGo applies the user's system font scale.
// Keep secondary labels readable instead of shrinking the whole interface.
const (
	phoneFontDisplay     core.Points = 22
	phoneFontTitle       core.Points = 20
	phoneFontSection     core.Points = 17
	phoneFontSong        core.Points = 15
	phoneFontBody        core.Points = 14
	phoneFontCaption     core.Points = 12
	phoneFontLyrics      core.Points = 21
	phoneFontTranslation core.Points = 13
)

// Two-line rows grow with accessibility text while retaining a 48-point target.
func phoneTwoLineHeight(cx *core.Scope, title core.Points) core.Points {
	scale := max(float64(1), float64(cx.Env().FontScale()))
	return max(64, core.Points(math.Ceil(float64(title+phoneFontCaption)*1.35*scale+20)))
}

func (a *app) headingSize(desktop, mobile core.Points) core.Points {
	if a.phoneLayout(a.cx) {
		return mobile
	}
	return desktop
}
func (a *app) headingWeight() core.FontWeight {
	if a.phoneLayout(a.cx) {
		return core.FontWeightSemibold
	}
	return core.FontWeightBold
}
