package acquire

import (
	"math"
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

// delayProfiles is the setup of Radarr's DelayProfileServiceFixture: the
// default profile last and three others.
func delayProfiles() []DelayProfile {
	return []DelayProfile{
		{ID: tid(1), Default: true, Order: math.MaxInt},
		{ID: tid(2), Order: 1},
		{ID: tid(3), Order: 2},
		{ID: tid(4), Order: 3},
	}
}

func byOrder(ps []DelayProfile) []DelayProfile {
	ps = slices.Clone(ps)
	slices.SortStableFunc(ps, func(a, b DelayProfile) int { return cmpInt(a.Order, b.Order) })
	return ps
}

// TestReorderDelayProfiles ports Radarr's DelayProfileServiceFixture.
func TestReorderDelayProfiles(t *testing.T) {
	t.Run("first when no after", func(t *testing.T) {
		ps := delayProfiles()
		ReorderDelayProfiles(ps, tid(4), core.ID{})
		got := byOrder(ps)
		if got[0].ID != tid(4) || got[0].Order != 1 {
			t.Errorf("first = %+v, want = ID 4 at 1", got[0])
		}
		for i, p := range got[1:] {
			want := i + 2
			if p.Default {
				want = math.MaxInt
			}
			if p.Order != want {
				t.Errorf("profile %s order = %d, want = %d", p.ID, p.Order, want)
			}
		}
	})
	t.Run("after a profile", func(t *testing.T) {
		ps := delayProfiles()
		ReorderDelayProfiles(ps, tid(4), tid(2))
		got := byOrder(ps)
		if got[1].ID != tid(4) || got[1].Order != 2 {
			t.Errorf("second = %+v, want = ID 4 at 2", got[1])
		}
		if got[0].ID != tid(2) || got[0].Order != 1 {
			t.Errorf("first = %+v, want = ID 2 at 1 unchanged", got[0])
		}
	})
	t.Run("after a later profile", func(t *testing.T) {
		ps := delayProfiles()
		ReorderDelayProfiles(ps, tid(2), tid(4))
		i := slices.IndexFunc(ps, func(p DelayProfile) bool { return p.ID == tid(4) })
		if ps[i].Order >= 3 {
			t.Errorf("after's order = %d, want < 3", ps[i].Order)
		}
	})
	if ReorderDelayProfiles(delayProfiles(), tid(9), core.ID{}) {
		t.Error("ReorderDelayProfiles(unknown) = true, want = false")
	}
}

func TestBestDelayProfile(t *testing.T) {
	ps := []DelayProfile{
		{ID: tid(1), Default: true},
		{ID: tid(2), Order: 2, Tags: []string{"5"}},
		{ID: tid(3), Order: 1, Tags: []string{"6"}},
	}
	tests := []struct {
		tags []string
		want int
	}{
		{nil, 1},
		{[]string{"5"}, 2},
		{[]string{"5", "6"}, 3},
		{[]string{"7"}, 1},
	}
	for _, tt := range tests {
		if got := BestDelayProfile(ps, tt.tags); got == nil || got.ID != tid(tt.want) {
			t.Errorf("BestDelayProfile(%v) = %+v, want = ID %d", tt.tags, got, tt.want)
		}
	}
	if got := (&DelayProfile{TorrentDelay: 2, UsenetDelay: 1}).Delay(ProtocolTorrent); got != 2 {
		t.Errorf("Delay(torrent) = %v, want = 2", got)
	}
}
