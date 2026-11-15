package app

import (
	"math"

	"github.com/nsgo-dev/nsgo/core"
)

// Mobile sizes are logical points. NSGo applies the user's system font scale.
// Compact editorial sizes, calibrated against the phone's music apps.
const (
	phoneFontDisplay     core.Points = 18
	phoneFontHeader      core.Points = 17
	phoneFontTitle       core.Points = 18
	phoneFontSection     core.Points = 14
	phoneFontSong        core.Points = 13
	phoneFontBody        core.Points = 12
	phoneFontCaption     core.Points = 11
	phoneFontLyrics      core.Points = 19
	phoneFontTranslation core.Points = 11.5
)

// Two-line rows grow with accessibility text while retaining a 48-point target.
func phoneTwoLineHeight(cx *core.Scope, title core.Points) core.Points {
	scale := max(float64(1), float64(cx.Env().FontScale()))
	return max(58, core.Points(math.Ceil(float64(title+phoneFontCaption)*1.3*scale+18)))
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
