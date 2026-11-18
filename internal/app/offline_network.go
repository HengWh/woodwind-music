package app

import "github.com/nsgo-dev/nsgo/core"

func (a *app) configureOfflineNetwork() {
	if a.disk == nil {
		return
	}
	if a.cx.Env().OS() == core.OSAndroid {
		a.disk.Network(!a.set.OfflineMobile, a.offlineUnmetered.Load)
	} else {
		a.disk.Network(false, nil)
	}
}
func (a *app) setupOfflineNetwork() {
	if a.disk == nil {
		return
	}
	initial, _ := a.cx.HostParams()["music.unmetered"].(bool)
	a.offlineUnmetered.Store(initial)
	a.configureOfflineNetwork()
	a.cx.OnHostEvent("android.network", func(data any) {
		if m, ok := data.(map[string]any); ok {
			v, _ := m["unmetered"].(bool)
			a.offlineUnmetered.Store(v)
			go a.disk.ResumeWaiting()
		}
	})
	go a.disk.ResumeAfterRestart()
}
