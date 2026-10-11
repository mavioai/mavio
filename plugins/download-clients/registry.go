package main

import (
	"slices"
	"strings"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/aria2"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/blackhole"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/deluge"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/downloadstation"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/flood"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/freebox"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/hadouken"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/nzbget"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/nzbvortex"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/pneumatic"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/qbittorrent"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/rqbit"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/rtorrent"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/sabnzbd"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/transmission"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/utorrent"
)

// definitions returns the definitions of the clients, by name.
func definitions() []client.Definition {
	defs := slices.Concat(
		aria2.Definitions(),
		blackhole.Definitions(),
		deluge.Definitions(),
		downloadstation.Definitions(),
		flood.Definitions(),
		freebox.Definitions(),
		hadouken.Definitions(),
		nzbget.Definitions(),
		nzbvortex.Definitions(),
		pneumatic.Definitions(),
		qbittorrent.Definitions(),
		rqbit.Definitions(),
		rtorrent.Definitions(),
		sabnzbd.Definitions(),
		transmission.Definitions(),
		utorrent.Definitions(),
	)
	slices.SortFunc(defs, func(a, b client.Definition) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return defs
}
