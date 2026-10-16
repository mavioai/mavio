package rpc

import (
	"context"
	"fmt"
	"slices"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

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
	seriesTypesFromProto    = reverse(seriesTypes)
	displayOrdersFromProto  = reverse(displayOrders)
)

// MetadataService implements mavio.library.v1.MetadataService for
// administrators.
type MetadataService struct {
	store     core.Store
	refresher *library.Refresher
	// fetch downloads images.
	fetch func(ctx context.Context, url string) ([]byte, error)
	now   func() time.Time
	// Lyrics finds lyrics through the lyrics providers; nil finds none.
	Lyrics *library.Lyrics
	// ExternalIDKinds lists the kinds of external IDs; nil lists the
	// built-in kinds.
	ExternalIDKinds func() []metadata.ExternalIDKind
}

var _ libraryv1connect.MetadataServiceHandler = (*MetadataService)(nil)

// NewMetadataService returns a MetadataService changing items through
// refresher; fetch downloads images.
func NewMetadataService(store core.Store, refresher *library.Refresher, fetch func(ctx context.Context, url string) ([]byte, error)) *MetadataService {
	return &MetadataService{store: store, refresher: refresher, fetch: fetch, now: time.Now}
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
	out, err := itemsToProto(ctx, s.store, []core.Item{it}, true, core.NilID)
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
	before := it
	it, err = s.refresher.Update(ctx, lib, it.ID, func(it *core.Item) error {
		for _, p := range paths {
			itemSetters[p](it, md)
		}
		return nil
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	// A series monitored or numbered anew lists its episodes again now
	// rather than at its next refresh.
	if it.Kind == core.KindSeries && (it.Monitored != before.Monitored || it.DisplayOrder != before.DisplayOrder) {
		job := library.RefreshJob(it.ID, s.now())
		if _, err := s.store.Jobs().Enqueue(ctx, &job); err != nil {
			return nil, connectError(ctx, err)
		}
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
	"monitored": func(it *core.Item, md *libraryv1.ItemMetadata) { it.Monitored = md.GetMonitored() },
	"series_type": func(it *core.Item, md *libraryv1.ItemMetadata) {
		it.SeriesType = seriesTypesFromProto[md.GetSeriesType()]
	},
	"display_order": func(it *core.Item, md *libraryv1.ItemMetadata) {
		it.DisplayOrder = displayOrdersFromProto[md.GetDisplayOrder()]
	},
	"absolute_number": func(it *core.Item, md *libraryv1.ItemMetadata) {
		it.AbsoluteNumber = intOf(md.HasAbsoluteNumber(), md.GetAbsoluteNumber())
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
	return libraryv1.SetItemImageResponse_builder{Image: imagesToProto(ctx, []core.Image{img})[0]}.Build(), nil
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

// SearchRemoteLyrics asks the lyrics providers for lyrics of a track.
func (s *MetadataService) SearchRemoteLyrics(ctx context.Context, req *libraryv1.SearchRemoteLyricsRequest) (*libraryv1.SearchRemoteLyricsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	resp := &libraryv1.SearchRemoteLyricsResponse{}
	if s.Lyrics == nil {
		return resp, nil
	}
	it, err := s.store.Items().Get(ctx, core.MustParseID(req.GetItemId()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	lib, err := s.store.Libraries().Get(ctx, it.LibraryID)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	found, err := s.Lyrics.Search(ctx, lib, it.ID)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	for _, l := range found {
		rl := libraryv1.RemoteLyrics_builder{
			Provider: &l.Provider, Id: &l.ID, Name: &l.Name, Artists: l.Artists, Album: &l.Album, Synced: &l.Synced, Score: &l.Score,
		}.Build()
		if l.Duration > 0 {
			rl.SetDuration(durationpb.New(l.Duration))
		}
		resp.SetLyrics(append(resp.GetLyrics(), rl))
	}
	return resp, nil
}

// DownloadRemoteLyrics saves lyrics a lyrics provider found beside the
// track.
func (s *MetadataService) DownloadRemoteLyrics(ctx context.Context, req *libraryv1.DownloadRemoteLyricsRequest) (*libraryv1.DownloadRemoteLyricsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	notFound := connect.NewError(connect.CodeNotFound, fmt.Errorf("lyrics provider %s: %w", req.GetProvider(), core.ErrNotFound))
	if s.Lyrics == nil {
		return nil, notFound
	}
	it, err := s.store.Items().Get(ctx, core.MustParseID(req.GetItemId()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	lib, err := s.store.Libraries().Get(ctx, it.LibraryID)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if err := s.Lyrics.Download(ctx, lib, it.ID, req.GetProvider(), req.GetId()); err != nil {
		return nil, connectError(ctx, err)
	}
	return &libraryv1.DownloadRemoteLyricsResponse{}, nil
}
