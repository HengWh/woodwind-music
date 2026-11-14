package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
	"rsc.io/qr"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/local"
	"github.com/HengWh/woodwind-music/internal/plugin"
)

type pluginInstaller struct {
	open, busy        bool
	repo, err, remove string
	pending           *plugin.Pending
	drafts            map[string]string
	generation        int
}

func (a *app) cancelPluginDialog() {
	if a.installer.busy {
		return
	}
	if p := a.installer.pending; p != nil {
		p.Discard()
	}
	a.installer.pending = nil
	a.installer.open = false
	a.installer.remove = ""
	a.bump()
}
func (a *app) beginInstall() { a.installer.open = true; a.installer.err = ""; a.bump() }
func (a *app) preparePlugin(work func() (*plugin.Pending, error)) {
	if a.installer.busy {
		return
	}
	a.installer.open = true
	a.installer.busy = true
	a.installer.err = ""
	a.bump()
	go func() {
		pending, err := work()
		a.post(func() {
			a.installer.busy = false
			if err != nil {
				a.installer.err = err.Error()
			} else if pending.Manifest.ID == local.ID {
				pending.Discard()
				a.installer.err = i18n.T("This plugin ID is reserved")
			} else {
				a.installer.pending = pending
			}
			a.bump()
		})
	}()
}
func (a *app) installRepo() {
	repo := strings.TrimSpace(a.installer.repo)
	if repo != "" {
		a.preparePlugin(func() (*plugin.Pending, error) { return plugin.Fetch(repo, a.pluginDir()) })
	}
}
func (a *app) pickPlugin() {
	a.post(func() {
		a.cx.PickFiles(core.PickFilesOptions{Types: []string{"application/zip", ".zip"}, Limit: 1}, func(files []core.File, err error) {
			if err != nil {
				a.say(err.Error())
				return
			}
			if len(files) == 0 {
				return
			}
			file := files[0]
			a.preparePlugin(func() (*plugin.Pending, error) {
				if file.Path != "" {
					return plugin.Open(file.Path, a.pluginDir())
				}
				// Android's document provider grants a reader, not a filesystem path.
				in, err := file.Open()
				if err != nil {
					return nil, err
				}
				defer in.Close()
				tmp, err := os.CreateTemp(a.dir, "plugin-*.zip")
				if err != nil {
					return nil, err
				}
				defer os.Remove(tmp.Name())
				n, err := io.Copy(tmp, io.LimitReader(in, (1<<30)+1))
				closeErr := tmp.Close()
				if err != nil {
					return nil, err
				}
				if closeErr != nil {
					return nil, closeErr
				}
				if n > 1<<30 {
					return nil, fmt.Errorf("plugin archive is too large")
				}
				return plugin.Open(tmp.Name(), a.pluginDir())
			})
		})
	})
}
func (a *app) confirmPluginInstall() {
	pending := a.installer.pending
	if pending == nil || a.installer.busy {
		return
	}
	a.installer.busy = true
	a.bump()
	go func() {
		err := pending.Install(a.pluginDir())
		var p *plugin.Plugin
		if err == nil {
			p, err = plugin.Load(filepath.Join(a.pluginDir(), pending.Manifest.ID), a.pluginDataDir())
		}
		a.post(func() {
			a.installer.pending = nil
			a.installer.busy = false
			if err != nil {
				a.installer.err = err.Error()
				a.bump()
				return
			}
			id := p.Info().ID
			wasActive := a.source().Info().ID == id
			replaced := false
			for i, old := range a.plugins {
				if old.Info().ID == id {
					old.Close()
					a.plugins[i] = p
					replaced = true
					break
				}
			}
			if !replaced {
				a.plugins = append(a.plugins, p)
			}
			a.registerPlugin(p)
			if wasActive {
				a.active = a.local
				a.activateSource(id)
			}
			a.installer.open = false
			a.bump()
			a.say(i18n.T("Plugin installed"))
		})
	}()
}
func (a *app) removePlugin(id string) {
	for i, p := range a.plugins {
		if p.Info().ID == id {
			if tr := a.pl.Current(); tr != nil && tr.Source == id {
				a.pl.Clear()
			}
			if a.source().Info().ID == id {
				a.switchSource(local.ID)
			}
			a.sources.Lock()
			delete(a.sources.items, id)
			a.sources.Unlock()
			delete(a.accounts, id)
			a.plugins = append(a.plugins[:i], a.plugins[i+1:]...)
			a.installer.remove = ""
			a.installer.open = false
			a.installer.busy = true
			a.bump()
			go func() {
				err := p.Remove(nil)
				a.post(func() {
					a.installer.busy = false
					if err != nil {
						a.say(err.Error())
					}
					a.bump()
				})
			}()
			return
		}
	}
}
func (a *app) pluginCategories() settingsCatDef {
	groups := []settingsGroupDef{{rows: []settingsRowDef{{label: i18n.T("Install a Plugin"), sub: i18n.T("Install from GitHub or a ZIP file"), control: func(p palette) core.Node {
		return a.settingsAction(p, i18n.T("Install"), "plugin-install", a.installer.busy, a.beginInstall)
	}}}}}
	for _, plug := range a.plugins {
		p := plug
		info := p.Info()
		a.loadAccount(p)
		state := a.accountState(p)
		sub := info.Version
		if state.account != nil {
			sub += " · " + i18n.Tf("Signed in as %s", state.account.Name)
		}
		rows := []settingsRowDef{{label: info.Name, sub: sub, control: func(pal palette) core.Node {
			return a.settingsAction(pal, i18n.T("Open"), "plugin-open-"+info.ID, false, func() {
				a.switchSource(info.ID)
				a.closeSettings()
				if a.settingsWin != nil {
					a.settingsWin.Hide()
				}
				a.showMain()
			})
		}}}
		for _, setting := range info.Settings {
			setting := setting
			rows = append(rows, settingsRowDef{label: setting.Label, sub: setting.Help, control: func(pal palette) core.Node {
				key := info.ID + "/" + setting.Key
				if a.installer.drafts == nil {
					a.installer.drafts = map[string]string{}
				}
				if _, ok := a.installer.drafts[key]; !ok {
					a.installer.drafts[key] = p.Setting(setting.Key)
				}
				return core.View{Style: core.Style{Direction: core.Column, Gap: 6, Width: core.Pt(a.headingSize(240, 260)), MaxWidth: core.Pct(100)}, Children: []core.Node{
					base.Input{TestID: "plugin-setting-" + key, Label: setting.Label, Value: func() string { return a.installer.drafts[key] }, OnChange: func(v string) { a.installer.drafts[key] = v }, Style: core.Style{Height: core.Pt(44), Padding: core.All(8), Background: pal.hover, Color: pal.text, CornerRadius: 8}},
					a.settingsAction(pal, i18n.T("Save"), "plugin-save-"+key, false, func() {
						value := a.installer.drafts[key]
						a.sourceAction(p, func() error { return p.SetSetting(setting.Key, value) }, nil)
					}),
				}}
			}})
			if setting.Link != "" {
				link := setting.Link
				rows = append(rows, settingsRowDef{label: setting.Label, control: func(pal palette) core.Node {
					return a.settingsAction(pal, i18n.T("Open"), "plugin-help-"+info.ID+"-"+setting.Key, false, func() { a.cx.OpenURL(link, nil) })
				}})
			}
		}
		if state.account == nil && p.Can("signIn") && (!p.Can("account") || state.asked) {
			rows = append(rows, settingsRowDef{label: i18n.T("Account"), control: func(pal palette) core.Node {
				return a.settingsAction(pal, i18n.T("Sign In"), "plugin-signin-"+info.ID, a.signingIn, func() { a.signInSource(p) })
			}})
		}
		if state.account != nil && p.Can("signOut") {
			rows = append(rows, settingsRowDef{label: i18n.T("Sign Out"), control: func(pal palette) core.Node {
				return a.settingsAction(pal, i18n.T("Sign Out"), "plugin-signout-"+info.ID, a.signingIn || state.signingOut, func() { a.signOutSource(p) })
			}})
		}
		if len(p.Permissions().Process) > 0 {
			sub := i18n.T("This plugin wants to start programs, which can do anything you can. Only allow it if you trust it.")
			if a.cx.Env().OS() == core.OSAndroid {
				sub = i18n.T("Plugins that start desktop programs are unavailable on Android")
			}
			rows = append(rows, settingsRowDef{label: i18n.T("Allow Programs"), sub: sub, control: func(pal palette) core.Node {
				label := i18n.T("Allow")
				if p.Granted() {
					label = i18n.T("Revoke")
				}
				return a.settingsAction(pal, label, "plugin-grant-"+info.ID, a.cx.Env().OS() == core.OSAndroid, func() { on := !p.Granted(); a.sourceAction(p, func() error { return p.Grant(on) }, nil) })
			}})
		}
		if o := p.Origin(); o != nil {
			rows = append(rows, settingsRowDef{label: i18n.T("Check for Updates"), sub: o.Repo, control: func(pal palette) core.Node {
				return a.settingsAction(pal, i18n.T("Check for Updates"), "plugin-update-"+info.ID, a.installer.busy, func() {
					a.preparePlugin(func() (*plugin.Pending, error) {
						rel, err := plugin.Latest(o.Repo)
						if err != nil {
							return nil, err
						}
						if !plugin.IsNewer(rel.Version, o.Version) {
							return nil, fmt.Errorf("%s", i18n.T("Up to date"))
						}
						return plugin.FetchRelease(rel, a.pluginDir())
					})
				})
			}})
		}
		rows = append(rows, settingsRowDef{label: i18n.T("Remove"), sub: i18n.T("Plugin settings are kept for reinstalling"), control: func(pal palette) core.Node {
			return a.settingsAction(pal, i18n.T("Remove…"), "plugin-remove-"+info.ID, a.installer.busy || a.signingIn, func() { a.installer.remove = info.ID; a.installer.open = true; a.bump() })
		}})
		groups = append(groups, settingsGroupDef{title: info.Name, rows: rows})
	}
	return settingsCatDef{id: "plugins", label: i18n.T("Plugins"), icon: lucide.Puzzle(), groups: groups}
}

func (a *app) pluginDialogs(cx *core.Scope, settings bool) core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		a.rev.Get()
		if settings && a.settingsInMain || !settings && a.settingsWin != nil && a.settingsWin.Visible() {
			return core.Group{}
		}
		p := a.palette(cx)
		in := &a.installer
		dialog := ui.Dialog{TestID: "plugin-dialog", Open: func() bool { return in.open }, OnOpenChange: func(open bool, _ base.Reason) {
			if !open {
				a.cancelPluginDialog()
			}
		}, Title: i18n.T("Install a Plugin")}
		if in.remove != "" {
			id := in.remove
			dialog.Title = i18n.T("Remove Plugin?")
			dialog.Description = i18n.T("Plugin settings are kept for reinstalling")
			dialog.Footer = []core.Node{ui.Button{Text: i18n.T("Cancel"), OnPress: a.cancelPluginDialog}, ui.Button{Text: i18n.T("Remove"), TestID: "plugin-remove-confirm", OnPress: func() { a.removePlugin(id) }}}
		} else if pending := in.pending; pending != nil {
			dialog.Title = i18n.Tf("Install %s?", pending.Manifest.Name+" "+pending.Manifest.Version)
			permissions := pending.Manifest.Permissions
			description := i18n.T("Asks for no network")
			if len(permissions.Network) > 0 {
				description = i18n.Tf("Can reach %s", strings.Join(permissions.Network, ", "))
			}
			if len(permissions.Process) > 0 {
				description += "\n" + i18n.T("This plugin wants to start programs, which can do anything you can. Only allow it if you trust it.")
			}
			dialog.Description = description
			dialog.Footer = []core.Node{ui.Button{Text: i18n.T("Cancel"), OnPress: a.cancelPluginDialog}, ui.Button{Text: i18n.T("Install"), TestID: "plugin-confirm", Disabled: func() bool { return in.busy }, OnPress: a.confirmPluginInstall}}
		} else {
			dialog.Children = []core.Node{base.Input{TestID: "plugin-repo", Label: i18n.T("Install from GitHub"), Placeholder: i18n.T("owner/repo or URL"), Value: func() string { return in.repo }, OnChange: func(v string) { in.repo = v }, OnSubmit: func(string) { a.installRepo() }, Style: core.Style{Height: core.Pt(40), Padding: core.All(8), Background: p.hover, Color: p.text, CornerRadius: 8}},
				a.settingsAction(p, i18n.T("Choose ZIP File…"), "plugin-pick", in.busy, a.pickPlugin)}
			dialog.Footer = []core.Node{ui.Button{Text: i18n.T("Cancel"), Disabled: func() bool { return in.busy }, OnPress: a.cancelPluginDialog}, ui.Button{Text: i18n.T("Install from GitHub"), TestID: "plugin-fetch", Disabled: func() bool { return in.busy }, OnPress: a.installRepo}}
		}
		if in.busy {
			dialog.Children = append(dialog.Children, text(i18n.T("Opening the plugin…"), 13, core.FontWeightNormal, p.muted))
		}
		if in.err != "" {
			dialog.Children = append(dialog.Children, text(in.err, 13, core.FontWeightNormal, p.accent))
		}
		sign := ui.Dialog{TestID: "plugin-signin", Title: i18n.T("Sign In"), Open: func() bool { return a.signingIn }, OnOpenChange: func(open bool, _ base.Reason) {
			if !open {
				a.cancelSignIn()
			}
		}, Description: a.signPrompt.Text, Footer: []core.Node{ui.Button{Text: i18n.T("Cancel"), OnPress: a.cancelSignIn}}}
		if a.signPrompt.QR != "" {
			if code, err := qr.Encode(a.signPrompt.QR, qr.M); err == nil {
				sign.Children = []core.Node{core.Image{Source: core.ImageSource{Data: code.PNG()}, Style: core.Style{Width: core.Pt(220), Height: core.Pt(220)}}}
			}
		}
		return core.Group{a.fitPhoneDialog(cx, p, dialog), a.fitPhoneDialog(cx, p, sign)}
	})
}
