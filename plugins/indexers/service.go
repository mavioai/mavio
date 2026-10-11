package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// ListDefinitions implements IndexerProviderService.
func (p *plugin) ListDefinitions(context.Context, *pluginv1.ListDefinitionsRequest) (*pluginv1.ListDefinitionsResponse, error) {
	p.mu.RLock()
	defs := p.defs
	p.mu.RUnlock()
	out := make([]*pluginv1.IndexerDefinition, 0, len(defs))
	for _, d := range defs {
		out = append(out, definitionProto(d))
	}
	return pluginv1.ListDefinitionsResponse_builder{Definitions: out}.Build(), nil
}

func definitionProto(d indexer.Definition) *pluginv1.IndexerDefinition {
	fields := make([]*pluginv1.SettingField, 0, len(d.Fields))
	for _, f := range d.Fields {
		var opts []*pluginv1.SettingFieldOption
		for _, o := range f.Options {
			opts = append(opts, pluginv1.SettingFieldOption_builder{ValueJson: proto.String(jsonValue(o.Value)), Label: proto.String(o.Label)}.Build())
		}
		fields = append(fields, pluginv1.SettingField_builder{
			Name: proto.String(f.Name), Label: proto.String(f.Label), HelpText: proto.String(f.HelpText),
			Type: fieldTypes[f.Type].Enum(), DefaultJson: proto.String(jsonValue(f.Default)),
			Options: opts, Advanced: proto.Bool(f.Advanced), Secret: proto.Bool(f.Secret),
		}.Build())
	}
	return pluginv1.IndexerDefinition_builder{
		Id: proto.String(d.ID), Name: proto.String(d.Name), Description: proto.String(d.Description),
		Protocol: protocols[d.Protocol].Enum(), Privacy: privacies[d.Privacy].Enum(),
		Language: proto.String(d.Language), Links: d.Links, Fields: fields,
	}.Build()
}

var (
	fieldTypes = map[indexer.FieldType]pluginv1.SettingFieldType{
		indexer.FieldText:        pluginv1.SettingFieldType_SETTING_FIELD_TYPE_TEXT,
		indexer.FieldNumber:      pluginv1.SettingFieldType_SETTING_FIELD_TYPE_NUMBER,
		indexer.FieldCheckbox:    pluginv1.SettingFieldType_SETTING_FIELD_TYPE_CHECKBOX,
		indexer.FieldSelect:      pluginv1.SettingFieldType_SETTING_FIELD_TYPE_SELECT,
		indexer.FieldMultiSelect: pluginv1.SettingFieldType_SETTING_FIELD_TYPE_MULTI_SELECT,
		indexer.FieldInfo:        pluginv1.SettingFieldType_SETTING_FIELD_TYPE_INFO,
	}
	protocols = map[indexer.Protocol]pluginv1.DownloadProtocol{
		indexer.ProtocolTorrent: pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT,
		indexer.ProtocolUsenet:  pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_USENET,
	}
	privacies = map[indexer.Privacy]pluginv1.IndexerPrivacy{
		indexer.Public:      pluginv1.IndexerPrivacy_INDEXER_PRIVACY_PUBLIC,
		indexer.SemiPrivate: pluginv1.IndexerPrivacy_INDEXER_PRIVACY_SEMI_PRIVATE,
		indexer.Private:     pluginv1.IndexerPrivacy_INDEXER_PRIVACY_PRIVATE,
	}
)

// capsTimeout bounds fetching an indexer's capabilities while listing.
const capsTimeout = 30 * time.Second

// ListIndexers implements IndexerProviderService: the configured
// indexers with their capabilities, those fetched from the indexer read
// once and kept by its site.
func (p *plugin) ListIndexers(ctx context.Context, _ *pluginv1.ListIndexersRequest) (*pluginv1.ListIndexersResponse, error) {
	p.mu.RLock()
	list := slices.Clone(p.indexers)
	p.mu.RUnlock()
	out := make([]*pluginv1.Indexer, len(list))
	var wg sync.WaitGroup
	for i, c := range list {
		wg.Go(func() {
			b := pluginv1.Indexer_builder{
				Id: proto.String(c.cfg.ID), Name: proto.String(c.ix.Name), DefinitionId: proto.String(c.def.ID),
				Protocol: protocols[c.ix.Site.Info().Protocol].Enum(), Privacy: privacies[c.def.Privacy].Enum(),
				Enabled: proto.Bool(c.cfg.enabled()), Priority: proto.Int32(int32(c.ix.Priority)),
			}
			if c.ix.Site.Info().Protocol == indexer.ProtocolTorrent {
				b.RequiredFlags = c.cfg.RequiredFlags
				b.MinimumSeeders = proto.Int32(int32(c.cfg.minimumSeeders()))
				if c.cfg.SeedRatio > 0 {
					b.SeedRatio = proto.Float64(c.cfg.SeedRatio)
				}
				if c.cfg.SeedTime > 0 {
					b.SeedTime = durationpb.New(time.Duration(c.cfg.SeedTime) * time.Minute)
				}
				if c.cfg.SeasonPackSeedTime > 0 {
					b.SeasonPackSeedTime = durationpb.New(time.Duration(c.cfg.SeasonPackSeedTime) * time.Minute)
				}
				b.RejectBlocklistedTorrentHashes = proto.Bool(c.cfg.RejectBlocklistedTorrentHashes)
			}
			b.Tags, b.MultiLanguages = c.cfg.Tags, c.cfg.MultiLanguages
			if until, failure, ok := p.statuses.Disabled(c.cfg.ID); ok {
				b.DisabledUntil, b.Failure = timestamppb.New(until), proto.String(failure)
			}
			cctx, cancel := context.WithTimeout(ctx, capsTimeout)
			defer cancel()
			caps, err := c.ix.Site.Capabilities(cctx, c.ix)
			if err != nil {
				if b.Failure == nil {
					b.Failure = proto.String(err.Error())
				}
			} else {
				b.Capabilities = capabilitiesProto(caps)
			}
			out[i] = b.Build()
		})
	}
	wg.Wait()
	return pluginv1.ListIndexersResponse_builder{Indexers: out}.Build(), nil
}

func capabilitiesProto(c *indexer.Capabilities) *pluginv1.IndexerCapabilities {
	params := func(t indexer.SearchType) []string {
		var out []string
		for _, p := range c.SupportedParams(t) {
			out = append(out, string(p))
		}
		return out
	}
	return pluginv1.IndexerCapabilities_builder{
		SearchParams: params(indexer.SearchBasic), TvSearchParams: params(indexer.SearchTV), MovieSearchParams: params(indexer.SearchMovie),
		Categories:   categoriesProto(c.Categories.Tree()),
		LimitDefault: proto.Int32(int32(c.LimitsDefault)), LimitMax: proto.Int32(int32(c.LimitsMax)),
		RawSearch: proto.Bool(c.RawSearch),
	}.Build()
}

func categoriesProto(cs []*indexer.Category) []*pluginv1.IndexerCategory {
	out := make([]*pluginv1.IndexerCategory, 0, len(cs))
	for _, c := range cs {
		out = append(out, pluginv1.IndexerCategory_builder{
			Id: proto.Int32(int32(c.ID)), Name: proto.String(c.Name), Subcategories: categoriesProto(c.Sub),
		}.Build())
	}
	return out
}

// testTimeout bounds a test.
const testTimeout = 2 * time.Minute

// TestIndexer implements IndexerProviderService. An indexer described by
// a definition and settings is tested without keeping its status.
func (p *plugin) TestIndexer(ctx context.Context, req *pluginv1.TestIndexerRequest) (*pluginv1.TestIndexerResponse, error) {
	var ix *indexer.Indexer
	if req.GetIndexerId() != "" && req.GetDefinitionId() == "" {
		c, err := p.indexer(req.GetIndexerId())
		if err != nil {
			return nil, err
		}
		ix = c.ix
	} else {
		id := req.GetIndexerId()
		if id == "" {
			id = "test"
		}
		statuses, err := indexer.NewStatuses("", time.Now)
		if err != nil {
			return nil, err
		}
		p.mu.RLock()
		ix, _, err = p.newIndexer(indexerConfig{ID: id, Definition: req.GetDefinitionId(), Settings: []byte(req.GetSettingsJson())}, p.cfg.Proxies)
		p.mu.RUnlock()
		if err != nil {
			return pluginv1.TestIndexerResponse_builder{Failure: proto.String(err.Error())}.Build(), nil
		}
		ix.Statuses = statuses
	}
	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()
	if err := ix.Test(ctx); err != nil {
		return pluginv1.TestIndexerResponse_builder{Failure: proto.String(err.Error())}.Build(), nil
	}
	return &pluginv1.TestIndexerResponse{}, nil
}

// searchTimeout bounds each indexer's search.
const searchTimeout = 90 * time.Second

// SearchReleases implements IndexerProviderService: the indexers are
// searched at once, those failing reported beside the releases of the
// others.
func (p *plugin) SearchReleases(ctx context.Context, req *pluginv1.SearchReleasesRequest) (*pluginv1.SearchReleasesResponse, error) {
	q, err := query(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	p.mu.RLock()
	var targets []*configured
	for _, c := range p.indexers {
		if len(req.GetIndexerIds()) > 0 && slices.Contains(req.GetIndexerIds(), c.cfg.ID) || len(req.GetIndexerIds()) == 0 && c.cfg.enabled() {
			targets = append(targets, c)
		}
	}
	p.mu.RUnlock()
	results := make([][]*pluginv1.Release, len(targets))
	failures := make([]*pluginv1.IndexerFailure, len(targets))
	var wg sync.WaitGroup
	for i, c := range targets {
		wg.Go(func() {
			sctx, cancel := context.WithTimeout(ctx, searchTimeout)
			defer cancel()
			qc := *q
			releases, err := c.ix.Search(sctx, &qc)
			if err != nil {
				p.log.WarnContext(ctx, "search failed", "indexer", c.cfg.ID, "err", err)
				f := pluginv1.IndexerFailure_builder{IndexerId: proto.String(c.cfg.ID), Message: proto.String(err.Error())}
				if until, _, ok := p.statuses.Disabled(c.cfg.ID); ok {
					f.DisabledUntil = timestamppb.New(until)
				}
				failures[i] = f.Build()
				return
			}
			for _, r := range releases {
				results[i] = append(results[i], releaseProto(c.cfg.ID, r))
			}
		})
	}
	wg.Wait()
	resp := &pluginv1.SearchReleasesResponse{}
	var releases []*pluginv1.Release
	for _, rs := range results {
		releases = append(releases, rs...)
	}
	resp.SetReleases(releases)
	var failed []*pluginv1.IndexerFailure
	for _, f := range failures {
		if f != nil {
			failed = append(failed, f)
		}
	}
	resp.SetFailures(failed)
	return resp, nil
}

// query returns the indexer query of a search.
func query(req *pluginv1.SearchReleasesRequest) (*indexer.Query, error) {
	q := &indexer.Query{
		Term: req.GetQuery(), Episode: req.GetEpisode(), Year: int(req.GetYear()), Genre: req.GetGenre(),
		Limit: int(req.GetLimit()), Offset: int(req.GetOffset()), Title: req.GetTitle(),
	}
	if req.HasMinAge() {
		q.MinAge = indexer.Ptr(int(req.GetMinAge()))
	}
	if req.HasMaxAge() {
		q.MaxAge = indexer.Ptr(int(req.GetMaxAge()))
	}
	if req.HasMinSize() {
		q.MinSize = indexer.Ptr(req.GetMinSize())
	}
	if req.HasMaxSize() {
		q.MaxSize = indexer.Ptr(req.GetMaxSize())
	}
	switch req.GetType() {
	case pluginv1.IndexerSearchType_INDEXER_SEARCH_TYPE_MOVIE:
		q.Type = indexer.SearchMovie
	case pluginv1.IndexerSearchType_INDEXER_SEARCH_TYPE_TV:
		q.Type = indexer.SearchTV
	}
	for _, c := range req.GetCategories() {
		q.Categories = append(q.Categories, int(c))
	}
	if len(q.Categories) == 0 {
		switch q.Type {
		case indexer.SearchMovie:
			q.Categories = []int{indexer.Movies.ID}
		case indexer.SearchTV:
			q.Categories = []int{indexer.TV.ID}
		}
	}
	if req.HasSeason() {
		q.Season = indexer.Ptr(int(req.GetSeason()))
	}
	for k, v := range req.GetExternalIds() {
		if k == "imdb" {
			q.IMDbID = strings.TrimPrefix(strings.ToLower(v), "tt")
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("external ID %s: %q is not a number", k, v)
		}
		switch k {
		case "tmdb":
			q.TMDBID = n
		case "tvdb":
			q.TVDBID = n
		case "tvrage":
			q.RID = n
		case "tvmaze":
			q.TVMazeID = n
		case "trakt":
			q.TraktID = n
		case "douban":
			q.DoubanID = n
		default:
			return nil, fmt.Errorf("unknown external ID %q", k)
		}
	}
	return q, nil
}

func releaseProto(indexerID string, r *indexer.Release) *pluginv1.Release {
	ids := map[string]string{}
	if r.IMDbID > 0 {
		ids["imdb"] = fmt.Sprintf("tt%07d", r.IMDbID)
	}
	for k, v := range map[string]int{"tmdb": r.TMDBID, "tvdb": r.TVDBID, "tvrage": r.TVRageID, "trakt": r.TraktID, "tvmaze": r.TVMazeID, "douban": r.DoubanID} {
		if v > 0 {
			ids[k] = strconv.Itoa(v)
		}
	}
	var cats []int32
	for _, c := range r.Categories {
		cats = append(cats, int32(c))
	}
	var flags []string
	for _, f := range r.Flags {
		flags = append(flags, string(f))
	}
	b := pluginv1.Release_builder{
		IndexerId: proto.String(indexerID), Guid: proto.String(r.GUID), Title: proto.String(r.Title),
		Description: proto.String(r.Description), DownloadUrl: proto.String(r.DownloadURL),
		MagnetUrl: proto.String(r.MagnetURL), InfoHash: proto.String(r.InfoHash),
		InfoUrl: proto.String(r.InfoURL), CommentsUrl: proto.String(r.CommentsURL),
		Protocol: protocols[r.Protocol].Enum(), Categories: cats, ExternalIds: ids,
		Year: proto.Int32(int32(r.Year)), Flags: flags, Languages: r.Languages, Subtitles: r.Subtitles,
		Genres: r.Genres, PosterUrl: proto.String(r.PosterURL),
		Origin: proto.String(r.Origin), Source: proto.String(r.Source), Container: proto.String(r.Container),
		Codec: proto.String(r.Codec), Resolution: proto.String(r.Resolution),
	}
	if !r.PublishDate.IsZero() {
		b.PublishDate = timestamppb.New(r.PublishDate)
	}
	if r.Size != nil {
		b.Size = r.Size
	}
	setInt := func(dst **int32, v *int) {
		if v != nil {
			*dst = proto.Int32(int32(*v))
		}
	}
	setInt(&b.Grabs, r.Grabs)
	setInt(&b.Files, r.Files)
	setInt(&b.Seeders, r.Seeders)
	setInt(&b.Peers, r.Peers)
	b.DownloadVolumeFactor = r.DownloadVolumeFactor
	b.UploadVolumeFactor = r.UploadVolumeFactor
	b.MinimumRatio = r.MinimumRatio
	b.MinimumSeedTime = r.MinimumSeedTime
	return b.Build()
}

// GetReleaseFile implements IndexerProviderService.
func (p *plugin) GetReleaseFile(ctx context.Context, req *pluginv1.GetReleaseFileRequest) (*pluginv1.GetReleaseFileResponse, error) {
	c, err := p.indexer(req.GetIndexerId())
	if err != nil {
		return nil, err
	}
	data, err := c.ix.Download(ctx, req.GetDownloadUrl())
	if err != nil {
		if errors.Is(err, indexer.ErrReleaseUnavailable) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	if bytes.HasPrefix(data, []byte("magnet:")) {
		return pluginv1.GetReleaseFileResponse_builder{MagnetUrl: proto.String(string(data))}.Build(), nil
	}
	contentType := "application/x-bittorrent"
	if c.ix.Site.Info().Protocol == indexer.ProtocolUsenet {
		contentType = "application/x-nzb"
	}
	return pluginv1.GetReleaseFileResponse_builder{Data: data, ContentType: proto.String(contentType)}.Build(), nil
}
