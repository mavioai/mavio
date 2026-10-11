package series

import (
	"context"
	"errors"
	"testing"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
)

func TestCatalog(t *testing.T) {
	aired := func(id, season, number int, date string) *Episode {
		e := ep(id, season, number)
		e.AirDate = date
		return e
	}
	a := &Series{Target: acquire.Target{ID: tid(1), Title: "The Series"}, TvdbID: 10, ImdbID: "tt0000010"}
	b := &Series{Target: acquire.Target{ID: tid(2), Title: "Series", Year: 2020}, TvdbID: 20}
	c := &Catalog{List: []*Series{a, b}, Load: func(_ context.Context, id core.ID) ([]*Episode, error) {
		if id != a.ID {
			return nil, nil
		}
		return []*Episode{
			aired(11, 1, 1, "2024-01-01"),
			aired(12, 1, 2, "2024-01-08"),
			aired(13, 1, 3, "2024-01-08"),
			aired(14, 0, 1, "2024-01-01"),
		}, nil
	}}
	ctx := t.Context()
	if s, err := c.FindByTitle(ctx, "The.Series"); err != nil || s != a {
		t.Errorf("FindByTitle = %v, %v, want = a", s, err)
	}
	if s, err := c.FindByTitleYear(ctx, "Series", 2020); err != nil || s != b {
		t.Errorf("FindByTitleYear = %v, %v, want = b", s, err)
	}
	if s, err := c.FindByTvdbID(ctx, 20); err != nil || s != b {
		t.Errorf("FindByTvdbID = %v, %v, want = b", s, err)
	}
	if s, err := c.FindByImdbID(ctx, "tt10"); err != nil || s != a {
		t.Errorf("FindByImdbID = %v, %v, want = a", s, err)
	}
	if s, err := c.FindByTvdbID(ctx, 0); err != nil || s != nil {
		t.Errorf("FindByTvdbID(0) = %v, %v, want = nil", s, err)
	}
	dup := &Catalog{List: []*Series{a, {Target: acquire.Target{Title: "The Series"}}}}
	if _, err := dup.FindByTitle(ctx, "The Series"); !errors.Is(err, ErrMultipleSeries) {
		t.Errorf("FindByTitle of two = %v, want = %v", err, ErrMultipleSeries)
	}
	tests := []struct {
		date    string
		part    *int
		want    core.ID
		wantErr error
	}{
		{"2024-01-01", nil, tid(11), nil}, // the regular one of a special and it
		{"2024-01-08", nil, core.ID{}, ErrMultipleEpisodes},
		{"2024-01-08", intPtr(2), tid(13), nil},
		{"2024-02-01", nil, core.ID{}, nil},
	}
	for _, tt := range tests {
		got, err := c.FindEpisodeByAirDate(ctx, a.ID, tt.date, tt.part)
		var id core.ID
		if got != nil {
			id = got.ID
		}
		if !errors.Is(err, tt.wantErr) || id != tt.want {
			t.Errorf("FindEpisodeByAirDate(%s, %v) = %v, %v, want = %v, %v", tt.date, tt.part, id, err, tt.want, tt.wantErr)
		}
	}
}
