package rpc

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"github.com/mavioai/mavio/apps/server/internal/acquisition"
	"github.com/mavioai/mavio/apps/server/internal/releases"
	"github.com/mavioai/mavio/libs/core"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/release"
)

// ReleaseService implements mavio.acquisition.v1.ReleaseService.
type ReleaseService struct {
	searcher *releases.Searcher
	decider  Decider
}

var _ acquisitionv1connect.ReleaseServiceHandler = (*ReleaseService)(nil)

// Decider decides on the releases a search found for a monitored movie,
// failing with acquisition.ErrNotWanted for other items, as
// acquisition.Runtime does.
type Decider interface {
	Decide(ctx context.Context, itemID core.ID, found []releases.Release) ([]acquisition.Decided, error)
}

// NewReleaseService returns a ReleaseService searching the indexers of
// the providers that providers returns, which may be nil, for the items of
// store, keeping them in cache unless nil; searches of monitored movies
// are decided on by decider, unless nil.
func NewReleaseService(store core.Store, providers func() []releases.Provider, cache *releases.IndexerCache, decider Decider) *ReleaseService {
	return &ReleaseService{searcher: &releases.Searcher{Providers: providers, Items: store.Items(), Cache: cache}, decider: decider}
}

// ListIndexers lists the indexers of every ready indexer provider.
func (s *ReleaseService) ListIndexers(ctx context.Context, _ *acquisitionv1.ListIndexersRequest) (*acquisitionv1.ListIndexersResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	indexers, failures := s.searcher.ListIndexers(ctx)
	out := make([]*acquisitionv1.Indexer, len(indexers))
	for i, ix := range indexers {
		caps := ix.GetCapabilities()
		out[i] = acquisitionv1.Indexer_builder{
			PluginId: &ix.Provider, Id: new(ix.GetId()), Name: new(ix.GetName()), DefinitionId: new(ix.GetDefinitionId()),
			Protocol: new(acquisitionv1.DownloadProtocol(ix.GetProtocol())), Privacy: new(acquisitionv1.IndexerPrivacy(ix.GetPrivacy())),
			Enabled: new(ix.GetEnabled()), Priority: new(ix.GetPriority()),
			SearchParams: caps.GetSearchParams(), TvSearchParams: caps.GetTvSearchParams(), MovieSearchParams: caps.GetMovieSearchParams(),
			DisabledUntil: ix.GetDisabledUntil(), Failure: new(ix.GetFailure()),
		}.Build()
	}
	return acquisitionv1.ListIndexersResponse_builder{Indexers: out, Failures: failuresToProto(failures)}.Build(), nil
}

// SearchReleases searches the indexers for an item or text.
func (s *ReleaseService) SearchReleases(ctx context.Context, req *acquisitionv1.SearchReleasesRequest) (*acquisitionv1.SearchReleasesResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	q := releases.Search{
		Type:       pluginv1.IndexerSearchType(req.GetType()),
		Query:      req.GetQuery(),
		Episode:    req.GetEpisode(),
		Categories: req.GetCategories(),
	}
	if req.HasItemId() {
		id, err := core.ParseID(req.GetItemId())
		if err != nil {
			return nil, connectError(ctx, err)
		}
		q.ItemID = id
	}
	if req.GetSeason() > 0 || req.HasSeason() && req.GetType() == acquisitionv1.ReleaseSearchType_RELEASE_SEARCH_TYPE_TV {
		q.Season = new(int(req.GetSeason()))
	}
	for _, r := range req.GetIndexers() {
		q.Indexers = append(q.Indexers, releases.Ref{Provider: r.GetPluginId(), Indexer: r.GetIndexerId()})
	}
	// Prowlarr's searches take the age and size limits; Radarr's and
	// Sonarr's do not.
	if req.HasMinAge() {
		q.MinAge = new(req.GetMinAge())
	}
	if req.HasMaxAge() {
		q.MaxAge = new(req.GetMaxAge())
	}
	if req.HasMinSize() {
		q.MinSize = new(req.GetMinSize())
	}
	if req.HasMaxSize() {
		q.MaxSize = new(req.GetMaxSize())
	}
	res, err := s.searcher.Search(ctx, q)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := s.decide(ctx, q.ItemID, res.Releases)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return acquisitionv1.SearchReleasesResponse_builder{Releases: out, Failures: failuresToProto(res.Failures)}.Build(), nil
}

// decide returns the releases of a search, decided on and best first for
// a monitored movie or a wanted series, season or episode, else as found.
func (s *ReleaseService) decide(ctx context.Context, itemID core.ID, found []releases.Release) ([]*acquisitionv1.Release, error) {
	if s.decider != nil && !itemID.IsZero() {
		decided, err := s.decider.Decide(ctx, itemID, found)
		switch {
		case err == nil:
			out := make([]*acquisitionv1.Release, len(decided))
			for i, d := range decided {
				r := releaseToProto(d.Release)
				rejections := make([]*acquisitionv1.Rejection, len(d.Rejections))
				for j, rj := range d.Rejections {
					rejections[j] = acquisitionv1.Rejection_builder{
						Reason: new(snakeCase(string(rj.Reason))), Message: new(rj.Message), Temporary: new(rj.Temporary),
					}.Build()
				}
				r.SetRejections(rejections)
				r.SetEvaluated(true)
				r.SetCustomFormats(d.Formats)
				r.SetCustomFormatScore(int32(d.Score))
				if len(d.EpisodeIDs) > 0 {
					ids := make([]string, len(d.EpisodeIDs))
					for j, id := range d.EpisodeIDs {
						ids[j] = id.String()
					}
					r.SetEpisodeIds(ids)
				}
				out[i] = r
			}
			return out, nil
		case !errors.Is(err, acquisition.ErrNotWanted):
			return nil, err
		}
	}
	out := make([]*acquisitionv1.Release, len(found))
	for i, r := range found {
		out[i] = releaseToProto(r)
	}
	return out, nil
}

// snakeCase returns a camel case code in snake case, as
// "qualityNotWanted" becomes "quality_not_wanted".
func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

func failuresToProto(failures []releases.Failure) []*acquisitionv1.IndexerFailure {
	out := make([]*acquisitionv1.IndexerFailure, len(failures))
	for i, f := range failures {
		out[i] = acquisitionv1.IndexerFailure_builder{
			PluginId: new(f.Provider), IndexerId: new(f.Indexer), Message: new(f.Message), DisabledUntil: timestamp(f.DisabledUntil),
		}.Build()
	}
	return out
}

func releaseToProto(r releases.Release) *acquisitionv1.Release {
	b := acquisitionv1.Release_builder{
		PluginId: new(r.Provider), IndexerId: new(r.GetIndexerId()), IndexerName: new(r.IndexerName),
		Guid: new(r.GetGuid()), Title: new(r.GetTitle()), DownloadUrl: new(r.GetDownloadUrl()), MagnetUrl: new(r.GetMagnetUrl()),
		InfoHash: new(r.GetInfoHash()), InfoUrl: new(r.GetInfoUrl()), Protocol: new(acquisitionv1.DownloadProtocol(r.GetProtocol())),
		PublishDate: r.GetPublishDate(), Categories: r.GetCategories(), ExternalIds: r.GetExternalIds(),
		Grabs: new(r.GetGrabs()), Seeders: new(r.GetSeeders()), Peers: new(r.GetPeers()),
		DownloadVolumeFactor: new(r.GetDownloadVolumeFactor()), UploadVolumeFactor: new(r.GetUploadVolumeFactor()),
		Flags: r.GetFlags(),
	}
	if r.HasSize() {
		b.Size = new(r.GetSize())
	}
	switch {
	case r.Movie != nil:
		m := r.Movie
		b.Parsed = acquisitionv1.ParsedRelease_builder{
			Kind: new(acquisitionv1.ReleaseKind_RELEASE_KIND_MOVIE), Titles: m.MovieTitles, Year: new(int32(m.Year)),
			Edition: new(m.Edition), ImdbId: new(m.ImdbID), TmdbId: new(int32(m.TmdbID)),
			Quality: qualityToProto(m.Quality), Languages: languageNames(m.Languages), ReleaseGroup: new(m.ReleaseGroup),
		}.Build()
	case r.Episode != nil:
		e := r.Episode
		b.Parsed = acquisitionv1.ParsedRelease_builder{
			Kind: new(acquisitionv1.ReleaseKind_RELEASE_KIND_EPISODE), Titles: nonEmpty(e.SeriesTitle), Year: new(int32(e.SeriesTitleInfo.Year)),
			Season: new(int32(e.SeasonNumber)), Episodes: int32s(e.EpisodeNumbers), AbsoluteEpisodes: int32s(e.AbsoluteEpisodeNumbers),
			AirDate: new(e.AirDate), FullSeason: new(e.FullSeason),
			Quality: qualityToProto(e.Quality), Languages: languageNames(e.Languages), ReleaseGroup: new(e.ReleaseGroup),
		}.Build()
	}
	return b.Build()
}

var sourceNames = map[release.Source]string{
	release.SourceCam: "cam", release.SourceTelesync: "telesync", release.SourceTelecine: "telecine",
	release.SourceWorkprint: "workprint", release.SourceDVD: "dvd", release.SourceTV: "tv", release.SourceWebDL: "webdl",
	release.SourceWebRip: "webrip", release.SourceBluray: "bluray",
}

var modifierNames = map[release.Modifier]string{
	release.ModifierRegional: "regional", release.ModifierScreener: "screener", release.ModifierRawHD: "rawhd",
	release.ModifierBRDisk: "brdisk", release.ModifierRemux: "remux",
}

func qualityToProto(q release.ParsedQuality) *acquisitionv1.ReleaseQuality {
	resolution := q.Resolution
	if resolution == 0 {
		resolution = q.Quality.Resolution()
	}
	return acquisitionv1.ReleaseQuality_builder{
		Name: new(q.Quality.String()), Source: new(sourceNames[q.Quality.Source()]), Resolution: new(int32(resolution)),
		Modifier: new(modifierNames[q.Quality.Modifier()]),
		Version:  new(int32(q.Revision.Version)), Real: new(int32(q.Revision.Real)), Repack: new(q.Revision.IsRepack),
	}.Build()
}

func languageNames(languages []release.Language) []string {
	out := make([]string, len(languages))
	for i, l := range languages {
		out[i] = l.String()
	}
	return out
}

func int32s(ns []int) []int32 {
	out := make([]int32, len(ns))
	for i, n := range ns {
		out[i] = int32(n)
	}
	return out
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}
