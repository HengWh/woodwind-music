package app

import (
	"errors"
	"path/filepath"

	"github.com/nsgo-dev/nsgo/core"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/local"
)

func (a *app) importFolders(dirs []core.Directory) {
	a.importing = true
	a.bump()
	a.say(i18n.T("Importing music…"))
	ctx := a.cx.Context()
	go func() {
		var imported []string
		var failures []error
		for _, d := range dirs {
			path, err := local.ImportFolder(ctx, d, filepath.Join(a.dir, "imports"), d.Name)
			if err != nil {
				failures = append(failures, err)
			} else {
				imported = append(imported, path)
			}
		}
		a.post(func() {
			a.importing = false
			if len(imported) > 0 {
				a.local.AddFolders(imported...)
				a.reload()
				a.open(route{page: pageBrowse, id: "songs"})
			}
			if err := errors.Join(failures...); err != nil {
				if errors.Is(err, local.ErrNoMusic) && len(failures) == 1 {
					a.say(i18n.T("No music found"))
				} else {
					a.say(err.Error())
				}
			} else {
				a.say(i18n.T("Music imported"))
			}
			a.bump()
		})
	}()
}
