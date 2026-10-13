package main

import (
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/providers/opensubtitlescom"
)

// definitions returns the providers the plugin implements.
func definitions() []provider.Definition {
	return []provider.Definition{
		opensubtitlescom.Definition(),
	}
}
