package providers

import (
	"context"
	"fmt"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/metadata"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// EpisodeListPlugin is an episode list provider plugin as a
// library.EpisodeLister.
type EpisodeListPlugin struct {
	ID     string
	Client pluginv1connect.EpisodeListProviderServiceClient
}

// Name returns the plugin's ID.
func (p *EpisodeListPlugin) Name() string { return p.ID }

// Episodes asks the plugin for a series' seasons and episodes; nil when it
// does not know the series.
func (p *EpisodeListPlugin) Episodes(ctx context.Context, l library.Lookup) (*library.EpisodeList, error) {
	resp, err := p.Client.ListEpisodes(ctx, pluginv1.ListEpisodesRequest_builder{
		Lookup: toLookup(pluginv1.MediaKind_MEDIA_KIND_SERIES, l),
	}.Build())
	if err != nil {
		return nil, fmt.Errorf("list episodes: %w", err)
	}
	if !resp.GetFound() || resp.GetProvider() == "" {
		return nil, nil
	}
	list := &library.EpisodeList{Provider: core.Provider(resp.GetProvider())}
	for _, s := range resp.GetSeasons() {
		res := listedResult(s.GetMetadata())
		res.Item.IndexNumber = new(int(s.GetSeasonNumber()))
		list.Seasons = append(list.Seasons, res)
	}
	for _, e := range resp.GetEpisodes() {
		res := listedResult(e.GetMetadata())
		res.Item.ParentIndexNumber = new(int(e.GetSeasonNumber()))
		res.Item.IndexNumber = new(int(e.GetEpisodeNumber()))
		if e.HasAbsoluteNumber() {
			res.Item.AbsoluteNumber = new(int(e.GetAbsoluteNumber()))
		}
		list.Episodes = append(list.Episodes, res)
	}
	return list, nil
}

func listedResult(md *pluginv1.Metadata) metadata.Result {
	if md == nil {
		return metadata.Result{}
	}
	return *toResult(md)
}
