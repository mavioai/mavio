package app

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// offlineProvider is the provider of remotes' offline downloads, built
// into the server.
const offlineProvider = "dev.mavio.rclone-offline"

// folderDraft is a download client's folder being edited: an added one
// names the client's folder too; one mounted in the client's managed
// container is not edited here.
type folderDraft struct {
	remote, local    string
	added, container bool
}

// folderDrafts are a download client's folders as edited, and the
// response they were drafted from.
type folderDrafts struct {
	from *acquisitionv1.GetDownloadClientPathsResponse
	list []folderDraft
}

func folderDraftsOf(paths []*acquisitionv1.DownloadClientPath) []folderDraft {
	out := make([]folderDraft, len(paths))
	for i, p := range paths {
		out[i] = folderDraft{remote: p.GetRemotePath(), local: p.GetLocalPath(), container: p.GetContainer()}
	}
	return out
}

// serverFolder is the pattern of where the server may read a folder: an
// absolute path or a remote's folder.
var serverFolder = regexp.MustCompile(`^(/|[A-Za-z]:[\\/]|\\\\)|^remote:[^/\\:]+:[^/]?`)

func validServerFolder(s string) bool {
	v := strings.TrimSpace(s)
	return v == "" || serverFolder.MatchString(v)
}

// downloadClientPage builds a download client's page, id being
// "<plugin>/<client>": a link to the plugin that set it up, or the remote
// whose offline downloads it is, and the folders the client puts
// downloads in, with where the server reads each, whether it can, and the
// libraries imports from it hard link to. A container's client takes no
// folders of its own: they are its container's.
func (a *App) downloadClientPage(c *ui.Context, id string) {
	st := &a.settingsState.admin.acquisition
	pluginID, clientID, _ := strings.Cut(id, "/")
	clients := adminTask(a, &st.clients, "download-clients", func(ctx context.Context, s string) (*acquisitionv1.ListDownloadClientsResponse, error) {
		return a.downloadService(s).ListDownloadClients(ctx, &acquisitionv1.ListDownloadClientsRequest{})
	})
	var client *acquisitionv1.DownloadClient
	for _, x := range clients.value.GetClients() {
		if x.GetPluginId() == pluginID && x.GetId() == clientID {
			client = x
		}
	}
	offline := pluginID == offlineProvider
	readOnly := client.GetContainer() != "" || offline
	a.backTo(c, style.T(c, "settings.acquisition"), acquisitionPath("", ""))
	sectionHeader(c, cmpOr(client.GetName(), clientID), style.T(c, "client.message"))
	t := adminTask(a, &st.paths, "client-paths/"+id, func(ctx context.Context, s string) (*acquisitionv1.GetDownloadClientPathsResponse, error) {
		return a.downloadService(s).GetDownloadClientPaths(ctx, acquisitionv1.GetDownloadClientPathsRequest_builder{PluginId: &pluginID, ClientId: &clientID}.Build())
	})
	group(c, "link", "", "", func(g *settingsGroup) {
		if offline {
			if g.link("remote", "cloud", ui.RGB(0x30, 0xB0, 0xC7), style.T(c, "client.remote"), clientID, true) {
				a.router.Push(storagePath(clientID))
			}
			return
		}
		name := pluginID
		for _, p := range st.plugins.value {
			if m := p.GetManifest(); m.GetId() == pluginID && m.GetName() != "" {
				name = m.GetName()
			}
		}
		if g.link("plugin", "plugins", ui.RGB(0x58, 0x56, 0xD6), style.T(c, "client.settings"), name, true) {
			a.router.Push(pluginsPath(pluginID, false))
		}
	})
	saved := t.value.GetPaths()
	if t.loaded && (st.folders == nil || st.folders.from != t.value && slices.Equal(st.folders.list, folderDraftsOf(st.folders.from.GetPaths()))) {
		st.folders = &folderDrafts{from: t.value, list: folderDraftsOf(saved)}
	}
	var d []folderDraft
	if st.folders != nil {
		d = st.folders.list
	}
	footer := "client.foldersFooter"
	switch {
	case offline:
		footer = "client.offlineFooter"
	case client.GetContainer() != "":
		footer = "client.containerFooter"
	}
	colors := style.Colors(c)
	group(c, "folders", style.T(c, "client.folders"), style.T(c, footer), func(g *settingsGroup) {
		for i := range d {
			x := &d[i]
			var p *acquisitionv1.DownloadClientPath
			if !x.added {
				for _, s := range saved {
					if s.GetRemotePath() == x.remote {
						p = s
					}
				}
			}
			fixed := x.container || readOnly
			unchanged := p != nil && x.local == p.GetLocalPath()
			g.divider()
			ui.Column(c.Key(fmt.Sprint("folder-", i))).Padding(12, 16).Gap(8).Children(func() {
				mono := func(s string) ui.Element {
					return style.Text(ui.Text(c, s), style.Type(c).Body).Font("SF Mono, Menlo, monospace").TextColor(style.Color(colors.TextPrimary)).Selectable()
				}
				if x.added {
					ui.TextInput(c, &x.remote).FillWidth().Label(style.T(c, "client.clientFolder")).Placeholder(style.T(c, "client.clientFolder"))
				} else {
					mono(x.remote)
				}
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					secondaryText(c, "→")
					if fixed {
						mono(x.local).Grow(1)
					} else {
						in := ui.TextInput(c, &x.local).Grow(1).Label(style.T(c, "client.serverFolder")).Placeholder(style.T(c, "client.readAt"))
						if !validServerFolder(x.local) {
							in.Border(1, style.Color(colors.Danger))
						}
					}
					if !fixed && (x.added || (p != nil && !p.GetRootFolder())) && linkButton(c, style.T(c, "rules.remove")).Clicked() {
						st.folders.list = slices.Delete(slices.Clone(d), i, i+1)
					}
				})
				if p == nil || !unchanged {
					return
				}
				var parts []string
				if x.container {
					parts = append(parts, style.T(c, "client.containerFolder"))
				}
				if p.GetReadable() {
					how := "client.copies"
					switch {
					case len(p.GetLinkLibraryPaths()) > 0 && p.GetMoves():
						how = "client.moves"
					case len(p.GetLinkLibraryPaths()) > 0:
						how = "client.links"
					case p.GetMoves():
						how = "client.transfers"
					}
					ui.Row(c).Gap(4).Wrap().Children(func() {
						for _, s := range parts {
							secondaryText(c, s+" ·")
						}
						style.Text(ui.Text(c, style.T(c, "client.readable")), style.Type(c).Footnote).TextColor(style.Color(colors.Success))
						style.Text(ui.Text(c, "· "+style.T(c, how, "paths", strings.Join(p.GetLinkLibraryPaths(), ", "))), style.Type(c).Footnote).
							TextColor(style.Color(colors.TextSecondary))
					})
				} else {
					style.Text(ui.Text(c, strings.Join(append(parts, p.GetError()), " · ")), style.Type(c).Footnote).TextColor(style.Color(colors.Danger))
				}
				if !fixed && !p.GetReadable() && p.GetSuggestedPath() != "" && x.local == "" {
					if smallButton(c, style.T(c, "client.use", "path", p.GetSuggestedPath()), false) {
						x.local = p.GetSuggestedPath()
					}
				}
			})
		}
		switch {
		case t.err != nil:
			g.note(errorMessage(c, t.err), true)
		case t.value.GetFailure() != "":
			g.note(style.T(c, "client.unreported", "failure", t.value.GetFailure()), true)
		case t.loaded && len(d) == 0:
			g.note(style.T(c, "client.noFolders"), false)
		}
		if st.folders != nil && !readOnly && g.addRow(style.T(c, "client.addFolder")) {
			st.folders.list = append(slices.Clone(d), folderDraft{added: true})
		}
	})
	if st.folders == nil || readOnly {
		return
	}
	changed := !slices.Equal(d, folderDraftsOf(saved))
	valid := !slices.ContainsFunc(d, func(x folderDraft) bool { return strings.TrimSpace(x.remote) == "" || !validServerFolder(x.local) })
	a.acquisitionSave(c, style.T(c, "action.save"), changed && valid, changed, func() {
		var paths []*acquisitionv1.DownloadClientPath
		for _, x := range d {
			if !x.container && strings.TrimSpace(x.local) != "" {
				paths = append(paths, acquisitionv1.DownloadClientPath_builder{RemotePath: new(strings.TrimSpace(x.remote)), LocalPath: new(strings.TrimSpace(x.local))}.Build())
			}
		}
		a.acquisitionAction("save", func(ctx context.Context, server string) error {
			_, err := a.downloadService(server).UpdateDownloadClientPaths(ctx, acquisitionv1.UpdateDownloadClientPathsRequest_builder{
				PluginId: &pluginID, ClientId: &clientID, Paths: paths,
			}.Build())
			return err
		}, func() { st.folders = nil })
	})
}
