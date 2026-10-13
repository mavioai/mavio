package main

import (
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/animetosho"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/animetoshoxyz"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/bayflix"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/bsplayer"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/gestdown"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/napiprojekt"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/napisy24"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/opensubtitlescom"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/regielive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/subsynchro"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/subtis"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/subtitriid"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/subtitulamostv"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/supersubtitles"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/titrari"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/tsukihime"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/tvsubtitles"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/vladoonmooo"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/wizdom"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/yavkanet"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/yifysubtitles"
)

// definitions returns the providers the plugin implements, by name.
func definitions() []provider.Definition {
	return []provider.Definition{
		animetosho.Definition(),
		animetoshoxyz.Definition(),
		bayflix.Definition(),
		bsplayer.Definition(),
		gestdown.Definition(),
		napiprojekt.Definition(),
		napisy24.Definition(),
		opensubtitlescom.Definition(),
		regielive.Definition(),
		subsynchro.Definition(),
		subtis.Definition(),
		subtitriid.Definition(),
		subtitulamostv.Definition(),
		supersubtitles.Definition(),
		titrari.Definition(),
		tsukihime.Definition(),
		tvsubtitles.Definition(),
		vladoonmooo.Definition(),
		wizdom.Definition(),
		yavkanet.Definition(),
		yifysubtitles.Definition(),
	}
}
