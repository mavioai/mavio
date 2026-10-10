package rpc

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/metadata"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
)

var metadataFields = map[core.MetadataField]libraryv1.MetadataField{
	core.FieldCast:                libraryv1.MetadataField_METADATA_FIELD_CAST,
	core.FieldGenres:              libraryv1.MetadataField_METADATA_FIELD_GENRES,
	core.FieldProductionLocations: libraryv1.MetadataField_METADATA_FIELD_PRODUCTION_LOCATIONS,
	core.FieldStudios:             libraryv1.MetadataField_METADATA_FIELD_STUDIOS,
	core.FieldTags:                libraryv1.MetadataField_METADATA_FIELD_TAGS,
	core.FieldName:                libraryv1.MetadataField_METADATA_FIELD_NAME,
	core.FieldOverview:            libraryv1.MetadataField_METADATA_FIELD_OVERVIEW,
	core.FieldRuntime:             libraryv1.MetadataField_METADATA_FIELD_RUNTIME,
	core.FieldOfficialRating:      libraryv1.MetadataField_METADATA_FIELD_OFFICIAL_RATING,
}

// reverse inverts a one-to-one map.
func reverse[K, V comparable](m map[K]V) map[V]K {
	out := make(map[V]K, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

var (
	metadataFieldsFromProto = reverse(metadataFields)
	imageKindsFromProto     = reverse(imageKinds)
	seriesStatusesFromProto = reverse(seriesStatuses)
)

// MetadataService implements mavio.library.v1.MetadataService for
// administrators.
type MetadataService struct {
	store     core.Store
	refresher *library.Refresher
	subtitles *library.Subtitles
	// fetch downloads images.
	fetch func(ctx context.Context, url string) ([]byte, error)
	now   func() time.Time
	// Activity records subtitle downloads; nil records none.
	Activity *activity.Log
	// ExternalIDKinds lists the kinds of external IDs; nil lists the
	// built-in kinds.
	ExternalIDKinds func() []metadata.ExternalIDKind
}

var _ libraryv1connect.MetadataServiceHandler = (*MetadataService)(nil)

// NewMetadataService returns a MetadataService changing items through
// refresher and downloading subtitles through subtitles; fetch downloads
// images.
func NewMetadataService(store core.Store, refresher *library.Refresher, subtitles *library.Subtitles,
	fetch func(ctx context.Context, url string) ([]byte, error),
) *MetadataService {
	return &MetadataService{store: store, refresher: refresher, subtitles: subtitles, fetch: fetch, now: time.Now}
}

// ListExternalIdKinds lists the kinds of external IDs, of an item kind or
// of every kind.
func (s *MetadataService) ListExternalIdKinds(ctx context.Context, req *libraryv1.ListExternalIdKindsRequest) (*libraryv1.ListExternalIdKindsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	resp := &libraryv1.ListExternalIdKindsResponse{}
	for _, k := range externalIDKinds(s.ExternalIDKinds) {
		var kinds []libraryv1.ItemKind
		for ik := range k.Items {
			kinds = append(kinds, itemKinds[ik])
		}
		slices.Sort(kinds)
		if req.HasItemKind() && !slices.Contains(kinds, req.GetItemKind()) {
			continue
		}
		resp.SetKinds(append(resp.GetKinds(), libraryv1.ExternalIdKind_builder{
			Key: new(string(k.Provider)), Name: &k.Name, ItemKinds: kinds, Persons: &k.Persons, PluginId: &k.Plugin,
		}.Build()))
	}
	return resp, nil
}

// target returns the item an administrator names, with its library.
func (s *MetadataService) target(ctx context.Context, id string) (core.Item, core.Library, error) {
	if _, err := admin(ctx); err != nil {
		return core.Item{}, core.Library{}, err
	}
	it, err := s.store.Items().Get(ctx, core.MustParseID(id))
	if err != nil {
		return it, core.Library{}, connectError(ctx, err)
	}
	lib, err := s.store.Libraries().Get(ctx, it.LibraryID)
	if err != nil {
		return it, lib, connectError(ctx, err)
	}
	return it, lib, nil
}

// item returns an item as administrators see it.
func (s *MetadataService) item(ctx context.Context, it core.Item) (*libraryv1.Item, error) {
	out, err := itemsToProto(ctx, s.store, []core.Item{it}, true)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return out[0], nil
}

// placeholders queues the computation of an item's image placeholders.
func (s *MetadataService) placeholders(ctx context.Context, id core.ID) error {
	job := library.PlaceholdersJob(id, s.now())
	_, err := s.store.Jobs().Enqueue(ctx, &job)
	return err
}

// UpdateItem changes the fields of an item named by the update mask.
func (s *MetadataService) UpdateItem(ctx context.Context, req *libraryv1.UpdateItemRequest) (*libraryv1.UpdateItemResponse, error) {
	it, lib, err := s.target(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	md := req.GetMetadata()
	paths := req.GetUpdateMask().GetPaths()
	for _, p := range paths {
		if _, ok := itemSetters[p]; !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown field %q in the update mask", p))
		}
	}
	it, err = s.refresher.Update(ctx, lib, it.ID, func(it *core.Item) error {
		for _, p := range paths {
			itemSetters[p](it, md)
		}
		return nil
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := s.item(ctx, it)
	if err != nil {
		return nil, err
	}
	return libraryv1.UpdateItemResponse_builder{Item: out}.Build(), nil
}

// itemSetters set the item fields of each ItemMetadata field.
var itemSetters = map[string]func(*core.Item, *libraryv1.ItemMetadata){
	"name":           func(it *core.Item, md *libraryv1.ItemMetadata) { it.Name = md.GetName() },
	"sort_name":      func(it *core.Item, md *libraryv1.ItemMetadata) { it.SortName = md.GetSortName() },
	"original_title": func(it *core.Item, md *libraryv1.ItemMetadata) { it.OriginalTitle = md.GetOriginalTitle() },
	"overview":       func(it *core.Item, md *libraryv1.ItemMetadata) { it.Overview = md.GetOverview() },
	"tagline":        func(it *core.Item, md *libraryv1.ItemMetadata) { it.Tagline = md.GetTagline() },
	"production_year": func(it *core.Item, md *libraryv1.ItemMetadata) {
		it.ProductionYear = int(md.GetProductionYear())
	},
	"premiere_date": func(it *core.Item, md *libraryv1.ItemMetadata) {
		it.PremiereDate = timeOf(md.HasPremiereDate(), md.GetPremiereDate().AsTime)
	},
	"end_date": func(it *core.Item, md *libraryv1.ItemMetadata) {
		it.EndDate = timeOf(md.HasEndDate(), md.GetEndDate().AsTime)
	},
	"runtime":          func(it *core.Item, md *libraryv1.ItemMetadata) { it.Runtime = md.GetRuntime().AsDuration() },
	"official_rating":  func(it *core.Item, md *libraryv1.ItemMetadata) { it.OfficialRating = md.GetOfficialRating() },
	"custom_rating":    func(it *core.Item, md *libraryv1.ItemMetadata) { it.CustomRating = md.GetCustomRating() },
	"community_rating": func(it *core.Item, md *libraryv1.ItemMetadata) { it.CommunityRating = md.GetCommunityRating() },
	"critic_rating":    func(it *core.Item, md *libraryv1.ItemMetadata) { it.CriticRating = md.GetCriticRating() },
	"genres":           func(it *core.Item, md *libraryv1.ItemMetadata) { it.Genres = values(md.GetGenres()) },
	"tags":             func(it *core.Item, md *libraryv1.ItemMetadata) { it.Tags = values(md.GetTags()) },
	"studios":          func(it *core.Item, md *libraryv1.ItemMetadata) { it.Studios = values(md.GetStudios()) },
	"production_locations": func(it *core.Item, md *libraryv1.ItemMetadata) {
		it.ProductionLocations = values(md.GetProductionLocations())
	},
	"external_ids": func(it *core.Item, md *libraryv1.ItemMetadata) {
		it.ExternalIDs = nil
		for k, v := range md.GetExternalIds() {
			if k != "" && v != "" {
				if it.ExternalIDs == nil {
					it.ExternalIDs = map[core.Provider]string{}
				}
				it.ExternalIDs[core.Provider(k)] = v
			}
		}
	},
	"index_number": func(it *core.Item, md *libraryv1.ItemMetadata) {
		it.IndexNumber = intOf(md.HasIndexNumber(), md.GetIndexNumber())
	},
	"parent_index_number": func(it *core.Item, md *libraryv1.ItemMetadata) {
		it.ParentIndexNumber = intOf(md.HasParentIndexNumber(), md.GetParentIndexNumber())
	},
	"series_status": func(it *core.Item, md *libraryv1.ItemMetadata) {
		it.SeriesStatus = seriesStatusesFromProto[md.GetSeriesStatus()]
	},
	"artists":       func(it *core.Item, md *libraryv1.ItemMetadata) { it.Artists = values(md.GetArtists()) },
	"album_artists": func(it *core.Item, md *libraryv1.ItemMetadata) { it.AlbumArtists = values(md.GetAlbumArtists()) },
	"album":         func(it *core.Item, md *libraryv1.ItemMetadata) { it.Album = md.GetAlbum() },
	"locked":        func(it *core.Item, md *libraryv1.ItemMetadata) { it.Locked = md.GetLocked() },
	"locked_fields": func(it *core.Item, md *libraryv1.ItemMetadata) {
		it.LockedFields = nil
		for _, f := range md.GetLockedFields() {
			if field, ok := metadataFieldsFromProto[f]; ok && !slices.Contains(it.LockedFields, field) {
				it.LockedFields = append(it.LockedFields, field)
			}
		}
	},
}

// values drops empty and repeated values.
func values(in []string) []string {
	var out []string
	for _, v := range in {
		if v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

func timeOf(set bool, get func() time.Time) *time.Time {
	if !set {
		return nil
	}
	t := get().UTC()
	return &t
}

func intOf(set bool, n int32) *int {
	if !set {
		return nil
	}
	v := int(n)
	return &v
}

// RefreshItem refreshes an item now.
func (s *MetadataService) RefreshItem(ctx context.Context, req *libraryv1.RefreshItemRequest) (*libraryv1.RefreshItemResponse, error) {
	it, lib, err := s.target(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	it, err = s.refresher.RefreshWith(ctx, lib, it.ID, library.RefreshOptions{ReplaceMetadata: req.GetReplaceMetadata()})
	if err == nil {
		err = s.placeholders(ctx, it.ID)
	}
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := s.item(ctx, it)
	if err != nil {
		return nil, err
	}
	return libraryv1.RefreshItemResponse_builder{Item: out}.Build(), nil
}

// SearchRemote searches the providers for what an item may be.
func (s *MetadataService) SearchRemote(ctx context.Context, req *libraryv1.SearchRemoteRequest) (*libraryv1.SearchRemoteResponse, error) {
	it, lib, err := s.target(ctx, req.GetItemId())
	if err != nil {
		return nil, err
	}
	found, err := s.refresher.Search(ctx, lib, it.ID, library.SearchQuery{
		Name: req.GetName(), Year: int(req.GetYear()), ExternalIDs: providerIDs(req.GetExternalIds()), Provider: req.GetProvider(),
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*libraryv1.RemoteSearchResult, len(found))
	for i, f := range found {
		out[i] = libraryv1.RemoteSearchResult_builder{
			Provider: &f.Provider, Name: &f.Name, Year: new(int32(f.Year)), Overview: &f.Overview,
			ExternalIds: stringIDs(f.ExternalIDs), ImageUrl: &f.ImageURL, Score: &f.Score,
		}.Build()
	}
	return libraryv1.SearchRemoteResponse_builder{Results: out}.Build(), nil
}

func providerIDs(m map[string]string) map[core.Provider]string {
	var out map[core.Provider]string
	for k, v := range m {
		if k != "" && v != "" {
			if out == nil {
				out = map[core.Provider]string{}
			}
			out[core.Provider(k)] = v
		}
	}
	return out
}

func stringIDs(m map[core.Provider]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[string(k)] = v
	}
	return out
}

// IdentifyItem takes an item for what the external IDs name.
func (s *MetadataService) IdentifyItem(ctx context.Context, req *libraryv1.IdentifyItemRequest) (*libraryv1.IdentifyItemResponse, error) {
	it, lib, err := s.target(ctx, req.GetItemId())
	if err != nil {
		return nil, err
	}
	it, err = s.refresher.Identify(ctx, lib, it.ID, providerIDs(req.GetExternalIds()))
	if err == nil {
		err = s.placeholders(ctx, it.ID)
	}
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := s.item(ctx, it)
	if err != nil {
		return nil, err
	}
	return libraryv1.IdentifyItemResponse_builder{Item: out}.Build(), nil
}

// ListRemoteImages lists the images the providers have of an item.
func (s *MetadataService) ListRemoteImages(ctx context.Context, req *libraryv1.ListRemoteImagesRequest) (*libraryv1.ListRemoteImagesResponse, error) {
	it, lib, err := s.target(ctx, req.GetItemId())
	if err != nil {
		return nil, err
	}
	images, err := s.refresher.RemoteImages(ctx, lib, it.ID, imageKindsFromProto[req.GetKind()])
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*libraryv1.RemoteImage, len(images))
	for i, img := range images {
		out[i] = libraryv1.RemoteImage_builder{
			Provider: &img.Provider, Kind: new(imageKinds[img.Kind]), Url: &img.URL,
			Width: new(int32(img.Width)), Height: new(int32(img.Height)), Language: &img.Language, Score: &img.Score,
		}.Build()
	}
	return libraryv1.ListRemoteImagesResponse_builder{Images: out}.Build(), nil
}

// SetItemImage makes a downloaded or uploaded image an item's image.
func (s *MetadataService) SetItemImage(ctx context.Context, req *libraryv1.SetItemImageRequest) (*libraryv1.SetItemImageResponse, error) {
	it, lib, err := s.target(ctx, req.GetItemId())
	if err != nil {
		return nil, err
	}
	data := req.GetData()
	if req.HasUrl() {
		if data, err = s.fetch(ctx, req.GetUrl()); err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
	}
	img, err := s.refresher.SetImage(ctx, lib, it.ID, imageKindsFromProto[req.GetKind()], data)
	if err == nil {
		err = s.placeholders(ctx, it.ID)
	}
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.SetItemImageResponse_builder{Image: imagesToProto([]core.Image{img})[0]}.Build(), nil
}

// DeleteItemImage removes an image of an item.
func (s *MetadataService) DeleteItemImage(ctx context.Context, req *libraryv1.DeleteItemImageRequest) (*libraryv1.DeleteItemImageResponse, error) {
	it, lib, err := s.target(ctx, req.GetItemId())
	if err != nil {
		return nil, err
	}
	if err := s.refresher.DeleteImage(ctx, lib, it.ID, core.MustParseID(req.GetImageId())); err != nil {
		return nil, connectError(ctx, err)
	}
	return &libraryv1.DeleteItemImageResponse{}, nil
}

// sourceID parses an optional media source ID.
func sourceID(id string) core.ID {
	if id == "" {
		return core.ID{}
	}
	return core.MustParseID(id)
}

// SearchSubtitles searches the subtitle providers.
func (s *MetadataService) SearchSubtitles(ctx context.Context, req *libraryv1.SearchSubtitlesRequest) (*libraryv1.SearchSubtitlesResponse, error) {
	it, lib, err := s.target(ctx, req.GetItemId())
	if err != nil {
		return nil, err
	}
	found, err := s.subtitles.Search(ctx, lib, it.ID, sourceID(req.GetMediaSourceId()), req.GetLanguage(), req.GetHearingImpaired(), req.GetForced())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*libraryv1.RemoteSubtitle, len(found))
	for i, f := range found {
		out[i] = libraryv1.RemoteSubtitle_builder{
			Provider: &f.Provider, Id: &f.ID, Name: &f.Name, Language: &f.Language, Format: &f.Format,
			HearingImpaired: &f.HearingImpaired, Forced: &f.Forced, Score: &f.Score,
			DownloadCount: new(int32(f.DownloadCount)), HashMatch: &f.HashMatch,
		}.Build()
	}
	return libraryv1.SearchSubtitlesResponse_builder{Subtitles: out}.Build(), nil
}

// DownloadSubtitle saves a subtitle beside an item's video.
func (s *MetadataService) DownloadSubtitle(ctx context.Context, req *libraryv1.DownloadSubtitleRequest) (*libraryv1.DownloadSubtitleResponse, error) {
	it, lib, err := s.target(ctx, req.GetItemId())
	if err != nil {
		return nil, err
	}
	st, err := s.subtitles.Download(ctx, lib, it.ID, sourceID(req.GetMediaSourceId()), req.GetProvider(), req.GetSubtitleId())
	a := core.Activity{
		Type: "subtitle.downloaded", ItemID: it.ID, Title: "Downloaded a subtitle of " + it.Name,
		Attributes: map[string]string{"provider": req.GetProvider(), "subtitle": req.GetSubtitleId()},
	}
	if err != nil {
		a.Type, a.Severity, a.Title, a.Message = "subtitle.download_failed", core.SeverityError, "Failed to download a subtitle of "+it.Name, err.Error()
		s.Activity.Record(ctx, a)
	} else {
		a.Attributes["language"] = st.Language
		s.Activity.Publish(a)
	}
	if errors.Is(err, core.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	} else if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.DownloadSubtitleResponse_builder{Stream: streamToProto(&st, true)}.Build(), nil
}
