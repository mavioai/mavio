package main

import (
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/animetosho"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/animetoshoxyz"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/gestdown"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/opensubtitlescom"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/subtitriid"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/subtitulamostv"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/supersubtitles"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/titrari"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/tsukihime"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/tvsubtitles"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/yavkanet"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/yifysubtitles"
)

// definitions returns the providers the plugin implements, by name.
func definitions() []provider.Definition {
	return []provider.Definition{
		animetosho.Definition(),
		animetoshoxyz.Definition(),
		gestdown.Definition(),
		opensubtitlescom.Definition(),
		subtitriid.Definition(),
		subtitulamostv.Definition(),
		supersubtitles.Definition(),
		titrari.Definition(),
		tsukihime.Definition(),
		tvsubtitles.Definition(),
		yavkanet.Definition(),
		yifysubtitles.Definition(),
	}
}
