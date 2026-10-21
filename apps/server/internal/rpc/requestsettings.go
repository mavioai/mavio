package rpc

import (
	"context"

	"github.com/mavioai/mavio/libs/core"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
)

func requestSettingsToProto(r *core.RequestSettings) *acquisitionv1.RequestSettings {
	d := &r.DefaultPermissions
	b := acquisitionv1.RequestSettings_builder{
		DefaultRequestMovies: &d.Movies, DefaultRequestSeries: &d.Series,
		DefaultAutoApproveMovies: &d.AutoApproveMovies, DefaultAutoApproveSeries: &d.AutoApproveSeries,
		DefaultReportIssues: &d.ReportIssues,
		MovieQuotaLimit:     new(int32(r.MovieQuota.Limit)), MovieQuotaDays: new(int32(r.MovieQuota.Days)),
		SeriesQuotaLimit: new(int32(r.SeriesQuota.Limit)), SeriesQuotaDays: new(int32(r.SeriesQuota.Days)),
		MinimumAvailability: new(minimumAvailabilityToProto(r.MinimumAvailability)),
		SeasonFolder:        &r.SeasonFolder, MonitorNewItems: new(newItemMonitorToProto(r.MonitorNewItems)),
		Specials: &r.Specials, BlocklistedTags: r.BlocklistedTags, BlocklistedTagsLimit: new(int32(r.BlocklistedTagsLimit)),
	}
	for _, f := range []struct {
		id core.ID
		to **string
	}{
		{r.MovieLibraryID, &b.MovieLibraryId},
		{r.MovieQualityProfileID, &b.MovieQualityProfileId},
		{r.SeriesLibraryID, &b.SeriesLibraryId},
		{r.SeriesQualityProfileID, &b.SeriesQualityProfileId},
		{r.AnimeLibraryID, &b.AnimeLibraryId},
		{r.AnimeQualityProfileID, &b.AnimeQualityProfileId},
	} {
		if !f.id.IsZero() {
			*f.to = new(f.id.String())
		}
	}
	return b.Build()
}

func requestSettingsFromProto(p *acquisitionv1.RequestSettings) (core.RequestSettings, error) {
	out := core.RequestSettings{
		DefaultPermissions: core.RequestPermissions{
			Movies: p.GetDefaultRequestMovies(), Series: p.GetDefaultRequestSeries(),
			AutoApproveMovies: p.GetDefaultAutoApproveMovies(), AutoApproveSeries: p.GetDefaultAutoApproveSeries(),
			ReportIssues: p.GetDefaultReportIssues(),
		},
		MovieQuota:   core.Quota{Limit: int(p.GetMovieQuotaLimit()), Days: int(p.GetMovieQuotaDays())},
		SeriesQuota:  core.Quota{Limit: int(p.GetSeriesQuotaLimit()), Days: int(p.GetSeriesQuotaDays())},
		SeasonFolder: p.GetSeasonFolder(), Specials: p.GetSpecials(),
		BlocklistedTags: p.GetBlocklistedTags(), BlocklistedTagsLimit: int(p.GetBlocklistedTagsLimit()),
	}
	var err error
	if out.MinimumAvailability, err = minimumAvailability(p.GetMinimumAvailability()); err != nil {
		return out, err
	}
	out.MonitorNewItems = core.NewItemsNone
	if p.HasMonitorNewItems() {
		if out.MonitorNewItems, err = newItemMonitor(p.GetMonitorNewItems()); err != nil {
			return out, err
		}
	}
	for _, f := range []struct {
		s  string
		to *core.ID
	}{
		{p.GetMovieLibraryId(), &out.MovieLibraryID},
		{p.GetMovieQualityProfileId(), &out.MovieQualityProfileID},
		{p.GetSeriesLibraryId(), &out.SeriesLibraryID},
		{p.GetSeriesQualityProfileId(), &out.SeriesQualityProfileID},
		{p.GetAnimeLibraryId(), &out.AnimeLibraryID},
		{p.GetAnimeQualityProfileId(), &out.AnimeQualityProfileID},
	} {
		if f.s == "" {
			continue
		}
		if *f.to, err = core.ParseID(f.s); err != nil {
			return out, invalid(err)
		}
	}
	return out, nil
}

// GetRequestSettings returns the request settings.
func (s *RequestService) GetRequestSettings(ctx context.Context, _ *acquisitionv1.GetRequestSettingsRequest) (*acquisitionv1.GetRequestSettingsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	set := currentSettings(s.Settings)
	return acquisitionv1.GetRequestSettingsResponse_builder{Settings: requestSettingsToProto(&set.Requests)}.Build(), nil
}

// UpdateRequestSettings applies and stores the request settings.
func (s *RequestService) UpdateRequestSettings(ctx context.Context, req *acquisitionv1.UpdateRequestSettingsRequest) (*acquisitionv1.UpdateRequestSettingsResponse, error) {
	r, err := requestSettingsFromProto(req.GetSettings())
	if err != nil {
		return nil, err
	}
	set, err := changeSettings(ctx, s.Settings, s.Activity, "the request settings", func(cur *core.ServerSettings) error {
		cur.Requests = r
		return nil
	})
	if err != nil {
		return nil, err
	}
	return acquisitionv1.UpdateRequestSettingsResponse_builder{Settings: requestSettingsToProto(&set.Requests)}.Build(), nil
}
