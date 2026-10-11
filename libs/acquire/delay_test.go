package acquire

import (
	"math"
	"slices"
	"testing"
)

// delayProfiles is the setup of Radarr's DelayProfileServiceFixture: the
// default profile last and three others.
func delayProfiles() []DelayProfile {
	return []DelayProfile{
		{ID: 1, Order: math.MaxInt},
		{ID: 2, Order: 1},
		{ID: 3, Order: 2},
		{ID: 4, Order: 3},
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
		ReorderDelayProfiles(ps, 4, 0)
		got := byOrder(ps)
		if got[0].ID != 4 || got[0].Order != 1 {
			t.Errorf("first = %+v, want = ID 4 at 1", got[0])
		}
		for i, p := range got[1:] {
			want := i + 2
			if p.ID == DefaultDelayProfileID {
				want = math.MaxInt
			}
			if p.Order != want {
				t.Errorf("profile %d order = %d, want = %d", p.ID, p.Order, want)
			}
		}
	})
	t.Run("after a profile", func(t *testing.T) {
		ps := delayProfiles()
		ReorderDelayProfiles(ps, 4, 2)
		got := byOrder(ps)
		if got[1].ID != 4 || got[1].Order != 2 {
			t.Errorf("second = %+v, want = ID 4 at 2", got[1])
		}
		if got[0].ID != 2 || got[0].Order != 1 {
			t.Errorf("first = %+v, want = ID 2 at 1 unchanged", got[0])
		}
	})
	t.Run("after a later profile", func(t *testing.T) {
		ps := delayProfiles()
		ReorderDelayProfiles(ps, 2, 4)
		i := slices.IndexFunc(ps, func(p DelayProfile) bool { return p.ID == 4 })
		if ps[i].Order >= 3 {
			t.Errorf("after's order = %d, want < 3", ps[i].Order)
		}
	})
	if ReorderDelayProfiles(delayProfiles(), 9, 0) {
		t.Error("ReorderDelayProfiles(unknown) = true, want = false")
	}
}

func TestBestDelayProfile(t *testing.T) {
	ps := []DelayProfile{
		{ID: 1, Order: math.MaxInt},
		{ID: 2, Order: 2, Tags: []int{5}},
		{ID: 3, Order: 1, Tags: []int{6}},
	}
	tests := []struct {
		tags []int
		want int
	}{
		{nil, 1},
		{[]int{5}, 2},
		{[]int{5, 6}, 3},
		{[]int{7}, 1},
	}
	for _, tt := range tests {
		if got := BestDelayProfile(ps, tt.tags); got == nil || got.ID != tt.want {
			t.Errorf("BestDelayProfile(%v) = %+v, want = ID %d", tt.tags, got, tt.want)
		}
	}
	if got := (&DelayProfile{TorrentDelay: 2, UsenetDelay: 1}).Delay(ProtocolTorrent); got != 2 {
		t.Errorf("Delay(torrent) = %v, want = 2", got)
	}
}
