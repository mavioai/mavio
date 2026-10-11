package acquisition

import (
	"testing"

	"github.com/mavioai/mavio/libs/acquire/organize"
	"github.com/mavioai/mavio/libs/core"
)

func TestSeriesNamingSettings(t *testing.T) {
	s := core.DefaultAcquisitionSettings()
	s.Naming.RenameEpisodes = true
	s.Naming.SeasonFolderFormat = "S{season:00}"
	s.Naming.MultiEpisodeStyle = core.MultiEpisodeScene
	s.Naming.StandardEpisodeFormat = " "
	s.Naming.DailyEpisodeFormat = ""
	rn := &run{settings: s}
	got := rn.seriesNaming()
	def := organize.DefaultSeriesNaming()
	if !got.RenameEpisodes || got.SeasonFolderFormat != "S{season:00}" || got.MultiEpisodeStyle != organize.MultiEpisodeScene {
		t.Errorf("series naming = %+v, want = renaming, S{season:00} and the scene style", got)
	}
	if got.StandardEpisodeFormat != def.StandardEpisodeFormat || got.DailyEpisodeFormat != def.DailyEpisodeFormat {
		t.Errorf("formats = %q, %q, want = Sonarr's defaults", got.StandardEpisodeFormat, got.DailyEpisodeFormat)
	}
}
