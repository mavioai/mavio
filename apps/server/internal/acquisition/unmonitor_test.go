package acquisition

import (
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

func TestMissingEpisodesUnmonitored(t *testing.T) {
	e := newSeriesEnv(t)
	s := e.addSeries(81189, core.SeriesStandard)
	ep1, ep2 := e.episode(s.ID, 1, 1), e.episode(s.ID, 1, 2)
	other := core.Item{ID: core.NewID(), LibraryID: e.shows.ID, Kind: core.KindSeries, Name: "Other", Path: e.showsDir + "/Other", Monitored: true}
	loose := core.Item{
		ID: core.NewID(), LibraryID: e.shows.ID, ParentID: other.ID, Kind: core.KindEpisode, Name: "Loose",
		Path: e.showsDir + "/Other/Other S01E01.mkv", IndexNumber: new(1), ParentIndexNumber: new(1), Monitored: true,
	}
	for _, it := range []core.Item{other, loose} {
		if err := e.db.Items().Upsert(e.ctx, it); err != nil {
			t.Fatal(err)
		}
	}
	monitored := func(id core.ID) bool {
		it, err := e.db.Items().Get(e.ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return it.Monitored
	}

	// Off by default, as in Sonarr.
	e.rt.UnmonitorMissing(e.ctx, e.shows, []core.ID{ep1.ID})
	if !monitored(ep1.ID) {
		t.Fatal("episode unmonitored with the setting off")
	}

	e.settings.AutoUnmonitorPreviouslyDownloadedEpisodes = true
	e.rt.UnmonitorMissing(e.ctx, e.shows, []core.ID{ep1.ID, loose.ID, core.NewID()})
	if monitored(ep1.ID) {
		t.Error("missing episode of a wanted series monitored, want = not")
	}
	if !monitored(ep2.ID) {
		t.Error("present episode unmonitored, want = monitored")
	}
	if !monitored(loose.ID) {
		t.Error("missing episode of a series not wanted unmonitored, want = monitored")
	}
}
