package acquisition

import (
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

func TestAddDefaultProfiles(t *testing.T) {
	e := newEnv(t)
	names := func(kind core.ItemKind) []string {
		t.Helper()
		ps, err := e.db.QualityProfiles().List(e.ctx, kind)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, p := range ps {
			out = append(out, p.Name)
		}
		slices.Sort(out)
		return out
	}
	// The movies have a profile already; the series have none.
	for range 2 {
		if err := e.rt.AddDefaultProfiles(e.ctx); err != nil {
			t.Fatal(err)
		}
		if got, want := names(core.KindMovie), []string{"HD"}; !slices.Equal(got, want) {
			t.Errorf("movie profiles = %v, want = %v", got, want)
		}
		want := []string{"Any", "HD - 720p/1080p", "HD-1080p", "HD-720p", "SD", "Ultra-HD"}
		if got := names(core.KindSeries); !slices.Equal(got, want) {
			t.Errorf("series profiles = %v, want = %v", got, want)
		}
	}
}
