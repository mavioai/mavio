package rpc

import (
	"cmp"
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/browse"
	"github.com/mavioai/mavio/libs/core"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
)

var (
	itemKinds = map[core.ItemKind]libraryv1.ItemKind{
		core.KindMovie: libraryv1.ItemKind_ITEM_KIND_MOVIE, core.KindSeries: libraryv1.ItemKind_ITEM_KIND_SERIES,
		core.KindSeason: libraryv1.ItemKind_ITEM_KIND_SEASON, core.KindEpisode: libraryv1.ItemKind_ITEM_KIND_EPISODE,
		core.KindVideo: libraryv1.ItemKind_ITEM_KIND_VIDEO, core.KindMusicArtist: libraryv1.ItemKind_ITEM_KIND_MUSIC_ARTIST,
		core.KindMusicAlbum: libraryv1.ItemKind_ITEM_KIND_MUSIC_ALBUM, core.KindTrack: libraryv1.ItemKind_ITEM_KIND_TRACK,
		core.KindMusicVideo: libraryv1.ItemKind_ITEM_KIND_MUSIC_VIDEO, core.KindAudioBook: libraryv1.ItemKind_ITEM_KIND_AUDIOBOOK,
		core.KindBook: libraryv1.ItemKind_ITEM_KIND_BOOK, core.KindPhotoAlbum: libraryv1.ItemKind_ITEM_KIND_PHOTO_ALBUM,
		core.KindPhoto: libraryv1.ItemKind_ITEM_KIND_PHOTO, core.KindFolder: libraryv1.ItemKind_ITEM_KIND_FOLDER,
		core.KindCollection: libraryv1.ItemKind_ITEM_KIND_COLLECTION, core.KindPlaylist: libraryv1.ItemKind_ITEM_KIND_PLAYLIST,
	}
	extraKinds = map[core.ExtraKind]libraryv1.ExtraKind{
		core.ExtraTrailer: libraryv1.ExtraKind_EXTRA_KIND_TRAILER, core.ExtraClip: libraryv1.ExtraKind_EXTRA_KIND_CLIP,
		core.ExtraBehindTheScene: libraryv1.ExtraKind_EXTRA_KIND_BEHIND_THE_SCENES, core.ExtraDeletedScene: libraryv1.ExtraKind_EXTRA_KIND_DELETED_SCENE,
		core.ExtraInterview: libraryv1.ExtraKind_EXTRA_KIND_INTERVIEW, core.ExtraScene: libraryv1.ExtraKind_EXTRA_KIND_SCENE,
		core.ExtraSample: libraryv1.ExtraKind_EXTRA_KIND_SAMPLE, core.ExtraFeaturette: libraryv1.ExtraKind_EXTRA_KIND_FEATURETTE,
		core.ExtraShort: libraryv1.ExtraKind_EXTRA_KIND_SHORT, core.ExtraThemeSong: libraryv1.ExtraKind_EXTRA_KIND_THEME_SONG,
		core.ExtraThemeVideo: libraryv1.ExtraKind_EXTRA_KIND_THEME_VIDEO, core.ExtraOther: libraryv1.ExtraKind_EXTRA_KIND_OTHER,
	}
	seriesStatuses = map[core.SeriesStatus]libraryv1.SeriesStatus{
		core.SeriesContinuing: libraryv1.SeriesStatus_SERIES_STATUS_CONTINUING,
		core.SeriesEnded:      libraryv1.SeriesStatus_SERIES_STATUS_ENDED,
		core.SeriesUnreleased: libraryv1.SeriesStatus_SERIES_STATUS_UNRELEASED,
	}
	imageKinds = map[core.ImageKind]libraryv1.ImageKind{
		core.ImagePrimary: libraryv1.ImageKind_IMAGE_KIND_PRIMARY, core.ImageBackdrop: libraryv1.ImageKind_IMAGE_KIND_BACKDROP,
		core.ImageLogo: libraryv1.ImageKind_IMAGE_KIND_LOGO, core.ImageThumb: libraryv1.ImageKind_IMAGE_KIND_THUMB,
		core.ImageBanner: libraryv1.ImageKind_IMAGE_KIND_BANNER, core.ImageArt: libraryv1.ImageKind_IMAGE_KIND_ART,
		core.ImageDisc: libraryv1.ImageKind_IMAGE_KIND_DISC, core.ImageScreenshot: libraryv1.ImageKind_IMAGE_KIND_SCREENSHOT,
	}
	creditKinds = map[core.CreditKind]libraryv1.CreditKind{
		core.CreditActor: libraryv1.CreditKind_CREDIT_KIND_ACTOR, core.CreditGuestStar: libraryv1.CreditKind_CREDIT_KIND_GUEST_STAR,
		core.CreditDirector: libraryv1.CreditKind_CREDIT_KIND_DIRECTOR, core.CreditWriter: libraryv1.CreditKind_CREDIT_KIND_WRITER,
		core.CreditProducer: libraryv1.CreditKind_CREDIT_KIND_PRODUCER, core.CreditCreator: libraryv1.CreditKind_CREDIT_KIND_CREATOR,
		core.CreditComposer: libraryv1.CreditKind_CREDIT_KIND_COMPOSER, core.CreditConductor: libraryv1.CreditKind_CREDIT_KIND_CONDUCTOR,
		core.CreditLyricist: libraryv1.CreditKind_CREDIT_KIND_LYRICIST, core.CreditArtist: libraryv1.CreditKind_CREDIT_KIND_ARTIST,
		core.CreditAuthor: libraryv1.CreditKind_CREDIT_KIND_AUTHOR, core.CreditNarrator: libraryv1.CreditKind_CREDIT_KIND_NARRATOR,
		core.CreditOther: libraryv1.CreditKind_CREDIT_KIND_OTHER,
	}
	valueKinds = map[libraryv1.ValueKind]core.ValueKind{
		libraryv1.ValueKind_VALUE_KIND_GENRE: core.ValueGenre, libraryv1.ValueKind_VALUE_KIND_TAG: core.ValueTag,
		libraryv1.ValueKind_VALUE_KIND_STUDIO: core.ValueStudio, libraryv1.ValueKind_VALUE_KIND_ARTIST: core.ValueArtist,
		libraryv1.ValueKind_VALUE_KIND_YEAR: core.ValueYear,
	}
	sortFields = map[libraryv1.SortField]core.SortField{
		libraryv1.SortField_SORT_FIELD_NAME: core.SortName, libraryv1.SortField_SORT_FIELD_DATE_ADDED: core.SortDateAdded,
		libraryv1.SortField_SORT_FIELD_PREMIERE_DATE: core.SortPremiereDate, libraryv1.SortField_SORT_FIELD_PRODUCTION_YEAR: core.SortProductionYear,
		libraryv1.SortField_SORT_FIELD_COMMUNITY_RATING: core.SortCommunityRating, libraryv1.SortField_SORT_FIELD_RUNTIME: core.SortRuntime,
		libraryv1.SortField_SORT_FIELD_INDEX: core.SortIndex, libraryv1.SortField_SORT_FIELD_RANDOM: core.SortRandom,
		libraryv1.SortField_SORT_FIELD_LAST_PLAYED: core.SortLastPlayed, libraryv1.SortField_SORT_FIELD_PLAY_COUNT: core.SortPlayCount,
		libraryv1.SortField_SORT_FIELD_LIST_ORDER: core.SortListOrder,
	}
	streamKinds = map[core.StreamKind]libraryv1.StreamKind{
		core.StreamVideo: libraryv1.StreamKind_STREAM_KIND_VIDEO, core.StreamAudio: libraryv1.StreamKind_STREAM_KIND_AUDIO,
		core.StreamSubtitle: libraryv1.StreamKind_STREAM_KIND_SUBTITLE, core.StreamAttachment: libraryv1.StreamKind_STREAM_KIND_ATTACHMENT,
		core.StreamImage: libraryv1.StreamKind_STREAM_KIND_IMAGE, core.StreamData: libraryv1.StreamKind_STREAM_KIND_DATA,
	}
	videoRanges = map[core.VideoRangeType]libraryv1.VideoRange{
		core.RangeTypeSDR: libraryv1.VideoRange_VIDEO_RANGE_SDR, core.RangeTypeHDR10: libraryv1.VideoRange_VIDEO_RANGE_HDR10,
		core.RangeTypeHDR10Plus: libraryv1.VideoRange_VIDEO_RANGE_HDR10_PLUS, core.RangeTypeHLG: libraryv1.VideoRange_VIDEO_RANGE_HLG,
	}
)

// ItemService implements mavio.library.v1.ItemService. Users see the
// items their policy allows; only administrators see file paths.
type ItemService struct {
	store   core.Store
	browser *browse.Browser
}

var _ libraryv1connect.ItemServiceHandler = (*ItemService)(nil)

// defaultListLimit is the length of the latest and next up lists when the
// request does not set one.
const defaultListLimit = 20

// NewItemService returns an ItemService backed by store.
func NewItemService(store core.Store) *ItemService {
	return &ItemService{store: store, browser: browse.New(store)}
}

// GetItem returns an item with its media sources and credits.
func (s *ItemService) GetItem(ctx context.Context, req *libraryv1.GetItemRequest) (*libraryv1.GetItemResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.store.Items().Get(ctx, core.MustParseID(req.GetId()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if !p.User.CanAccess(&item) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such item"))
	}
	sources, err := s.store.MediaSources().ListForItem(ctx, item.ID)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	credits, err := s.store.People().CreditsForItem(ctx, item.ID)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	people := make([]core.Person, len(credits))
	owners := []core.ID{item.ID}
	for i, c := range credits {
		if people[i], err = s.store.People().Get(ctx, c.PersonID); err != nil {
			return nil, connectError(ctx, err)
		}
		owners = append(owners, c.PersonID)
	}
	images, err := s.store.Images().ListForOwners(ctx, owners)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := itemToProto(&item, images[item.ID], p.User.Admin)
	if err := newAncestry(s.store).describe(ctx, &item, out); err != nil {
		return nil, connectError(ctx, err)
	}

	resp := libraryv1.GetItemResponse_builder{Item: out}
	for i := range sources {
		resp.MediaSources = append(resp.MediaSources, mediaSourceToProto(&sources[i], p.User.Admin))
	}
	for i, c := range credits {
		resp.Credits = append(resp.Credits, libraryv1.Credit_builder{
			Person: personToProto(&people[i], images[c.PersonID]),
			Kind:   new(creditKinds[c.Kind]),
			Role:   &c.Role,
			Order:  new(int32(c.Order)),
		}.Build())
	}
	return resp.Build(), nil
}

// ListItems lists the items matching the request among those the caller
// may access.
func (s *ItemService) ListItems(ctx context.Context, req *libraryv1.ListItemsRequest) (*libraryv1.ListItemsResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	policy := &p.User.Policy
	q := core.ItemQuery{
		Recursive:     req.GetRecursive(),
		IncludeExtras: req.GetIncludeExtras(),
		Search:        req.GetSearch(),
		Genres:        req.GetGenres(),
		Tags:          req.GetTags(),
		Studios:       req.GetStudios(),
		YearFrom:      int(req.GetYearFrom()),
		YearTo:        int(req.GetYearTo()),
		MaxRating:     policy.MaxParentalRating,
		SkipUnrated:   policy.BlockUnrated,
		UserID:        p.User.ID,
		Resumable:     req.GetResumable(),
		Limit:         int(req.GetLimit()),
		Offset:        int(req.GetOffset()),
	}
	var ok bool
	if q.LibraryIDs, ok = libraryScope(policy, req.GetLibraryIds()); !ok {
		return &libraryv1.ListItemsResponse{}, nil
	}
	if req.HasParentId() {
		q.ParentID = core.MustParseID(req.GetParentId())
		// The items of a collection or playlist are linked into it and
		// listed in its order unless sorted otherwise.
		parent, err := s.store.Items().Get(ctx, q.ParentID)
		switch {
		case errors.Is(err, core.ErrNotFound) || err == nil && !p.User.CanAccess(&parent):
			return nil, connect.NewError(connect.CodeNotFound, errors.New("no such parent"))
		case err != nil:
			return nil, connectError(ctx, err)
		case parent.Kind.IsCurated():
			q.ParentID, q.MemberOf = core.NilID, parent.ID
			if len(req.GetSort()) == 0 {
				q.Sort = []core.SortSpec{{Field: core.SortListOrder}}
			}
		}
	}
	if req.HasPersonId() {
		q.PersonID = core.MustParseID(req.GetPersonId())
	}
	if req.HasPlayed() {
		q.Played = new(req.GetPlayed())
	}
	if req.HasFavorite() {
		q.Favorite = new(req.GetFavorite())
	}
	q.Kinds = itemKindsFromProto(req.GetKinds())
	for _, sort := range req.GetSort() {
		q.Sort = append(q.Sort, core.SortSpec{Field: sortFields[sort.GetField()], Desc: sort.GetDescending()})
	}
	if err := q.Validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	page, err := s.store.Items().Query(ctx, q)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := itemsToProto(ctx, s.store, page.Items, p.User.Admin)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.ListItemsResponse_builder{Items: out, Total: new(int32(page.Total))}.Build(), nil
}

// ListLatestItems lists the items added last.
func (s *ItemService) ListLatestItems(ctx context.Context, req *libraryv1.ListLatestItemsRequest) (*libraryv1.ListLatestItemsResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	scope, ok := browseScope(&p.User, req.GetLibraryIds())
	if !ok {
		return &libraryv1.ListLatestItemsResponse{}, nil
	}
	q := browse.LatestQuery{
		Kinds: itemKindsFromProto(req.GetKinds()),
		Group: !req.GetUngrouped(),
		Limit: cmp.Or(int(req.GetLimit()), defaultListLimit),
	}
	if req.HasPlayed() {
		q.Played = new(req.GetPlayed())
	}
	items, err := s.browser.Latest(ctx, scope, q)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := itemsToProto(ctx, s.store, items, p.User.Admin)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.ListLatestItemsResponse_builder{Items: out}.Build(), nil
}

// ListNextUp lists the next episodes of the series the caller has played.
func (s *ItemService) ListNextUp(ctx context.Context, req *libraryv1.ListNextUpRequest) (*libraryv1.ListNextUpResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	scope, ok := browseScope(&p.User, req.GetLibraryIds())
	if !ok {
		return &libraryv1.ListNextUpResponse{}, nil
	}
	q := browse.NextUpQuery{
		IncludeResumable: req.GetIncludeResumable(),
		Limit:            cmp.Or(int(req.GetLimit()), defaultListLimit),
		Offset:           int(req.GetOffset()),
	}
	if req.HasSeriesId() {
		q.SeriesID = core.MustParseID(req.GetSeriesId())
	}
	if req.HasSince() {
		q.Since = req.GetSince().AsTime()
	}
	items, err := s.browser.NextUp(ctx, scope, q)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := itemsToProto(ctx, s.store, items, p.User.Admin)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.ListNextUpResponse_builder{Items: out}.Build(), nil
}

// browseScope limits browsing to the requested libraries and the rating
// the user may see; it reports false when no library remains.
func browseScope(u *core.User, libraries []string) (browse.Scope, bool) {
	ids, ok := libraryScope(&u.Policy, libraries)
	return browse.Scope{
		UserID:      u.ID,
		LibraryIDs:  ids,
		MaxRating:   u.Policy.MaxParentalRating,
		SkipUnrated: u.Policy.BlockUnrated,
	}, ok
}

// itemsToProto describes items with their images, series, seasons and
// albums.
func itemsToProto(ctx context.Context, store core.Store, items []core.Item, admin bool) ([]*libraryv1.Item, error) {
	owners := make([]core.ID, len(items))
	for i := range items {
		owners[i] = items[i].ID
	}
	images, err := store.Images().ListForOwners(ctx, owners)
	if err != nil {
		return nil, err
	}
	anc := newAncestry(store)
	out := make([]*libraryv1.Item, len(items))
	for i := range items {
		out[i] = itemToProto(&items[i], images[items[i].ID], admin)
		if err := anc.describe(ctx, &items[i], out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ancestry looks up the series, seasons and albums of items once per
// response.
type ancestry struct {
	store core.Store
	items map[core.ID]*core.Item
}

func newAncestry(store core.Store) *ancestry {
	return &ancestry{store: store, items: map[core.ID]*core.Item{}}
}

func (a *ancestry) get(ctx context.Context, id core.ID) (*core.Item, error) {
	if id.IsZero() {
		return nil, nil
	}
	if it, ok := a.items[id]; ok {
		return it, nil
	}
	it, err := a.store.Items().Get(ctx, id)
	switch {
	case errors.Is(err, core.ErrNotFound):
		a.items[id] = nil
		return nil, nil
	case err != nil:
		return nil, err
	}
	a.items[id] = &it
	return &it, nil
}

// describe sets the series and season of an episode or season and the
// album of a track.
func (a *ancestry) describe(ctx context.Context, it *core.Item, out *libraryv1.Item) error {
	switch it.Kind {
	case core.KindEpisode, core.KindSeason, core.KindTrack:
	default:
		return nil
	}
	parent, err := a.get(ctx, it.ParentID)
	if err != nil || parent == nil {
		return err
	}
	switch {
	case it.Kind == core.KindTrack:
		if parent.Kind == core.KindMusicAlbum {
			out.SetAlbumId(parent.ID.String())
			out.SetAlbumName(parent.Name)
		}
		return nil
	case parent.Kind == core.KindSeason && it.Kind == core.KindEpisode:
		out.SetSeasonId(parent.ID.String())
		out.SetSeasonName(parent.Name)
		if parent, err = a.get(ctx, parent.ParentID); err != nil || parent == nil {
			return err
		}
	}
	if parent.Kind == core.KindSeries {
		out.SetSeriesId(parent.ID.String())
		out.SetSeriesName(parent.Name)
	}
	return nil
}

// GetPerson returns a person with their images.
func (s *ItemService) GetPerson(ctx context.Context, req *libraryv1.GetPersonRequest) (*libraryv1.GetPersonResponse, error) {
	if _, err := principal(ctx); err != nil {
		return nil, err
	}
	person, err := s.store.People().Get(ctx, core.MustParseID(req.GetId()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	images, err := s.store.Images().ListForOwner(ctx, person.ID)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.GetPersonResponse_builder{Person: personToProto(&person, images)}.Build(), nil
}

// ListValues lists attribute values of the items the caller may access.
func (s *ItemService) ListValues(ctx context.Context, req *libraryv1.ListValuesRequest) (*libraryv1.ListValuesResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	filter, ok := itemFilter(&p.User.Policy, req.GetLibraryIds(), req.GetItemKinds())
	if !ok {
		return &libraryv1.ListValuesResponse{}, nil
	}
	q := core.ValueQuery{
		Kind:   valueKinds[req.GetKind()],
		Items:  filter,
		Search: req.GetSearch(),
		Limit:  int(req.GetLimit()),
		Offset: int(req.GetOffset()),
	}
	if err := q.Validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	values, err := s.store.Items().Values(ctx, q)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*libraryv1.ValueCount, len(values))
	for i, v := range values {
		out[i] = libraryv1.ValueCount_builder{Value: &v.Value, ItemCount: new(int32(v.Count))}.Build()
	}
	return libraryv1.ListValuesResponse_builder{Values: out}.Build(), nil
}

// ListPeople lists the people credited on the items the caller may access.
func (s *ItemService) ListPeople(ctx context.Context, req *libraryv1.ListPeopleRequest) (*libraryv1.ListPeopleResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	filter, ok := itemFilter(&p.User.Policy, req.GetLibraryIds(), req.GetItemKinds())
	if !ok {
		return &libraryv1.ListPeopleResponse{}, nil
	}
	q := core.PersonQuery{Items: filter, Search: req.GetSearch(), Limit: int(req.GetLimit()), Offset: int(req.GetOffset())}
	for _, k := range req.GetCreditKinds() {
		for ck, pk := range creditKinds {
			if pk == k {
				q.CreditKinds = append(q.CreditKinds, ck)
			}
		}
	}
	people, err := s.store.People().Search(ctx, q)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	owners := make([]core.ID, len(people))
	for i := range people {
		owners[i] = people[i].Person.ID
	}
	images, err := s.store.Images().ListForOwners(ctx, owners)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*libraryv1.PersonCount, len(people))
	for i := range people {
		pc := &people[i]
		out[i] = libraryv1.PersonCount_builder{
			Person:    personToProto(&pc.Person, images[pc.Person.ID]),
			ItemCount: new(int32(pc.Count)),
		}.Build()
	}
	return libraryv1.ListPeopleResponse_builder{People: out}.Build(), nil
}

// libraryScope restricts the requested libraries, or all when none are
// requested, to those the policy allows: nil means every library. It
// reports false when no library remains.
func libraryScope(policy *core.UserPolicy, requested []string) ([]core.ID, bool) {
	if len(requested) == 0 {
		return policy.Libraries, policy.Libraries == nil || len(policy.Libraries) > 0
	}
	var ids []core.ID
	for _, id := range requested {
		if lib := core.MustParseID(id); policy.CanAccessLibrary(lib) {
			ids = append(ids, lib)
		}
	}
	return ids, len(ids) > 0
}

// itemFilter selects the items of the requested libraries and kinds that
// the policy allows; it reports false when no library remains.
func itemFilter(policy *core.UserPolicy, libraries []string, kinds []libraryv1.ItemKind) (core.ItemFilter, bool) {
	ids, ok := libraryScope(policy, libraries)
	return core.ItemFilter{
		LibraryIDs:  ids,
		Kinds:       itemKindsFromProto(kinds),
		MaxRating:   policy.MaxParentalRating,
		SkipUnrated: policy.BlockUnrated,
	}, ok
}

func itemKindsFromProto(kinds []libraryv1.ItemKind) []core.ItemKind {
	var out []core.ItemKind
	for _, k := range kinds {
		for ck, pk := range itemKinds {
			if pk == k {
				out = append(out, ck)
			}
		}
	}
	return out
}

func itemToProto(it *core.Item, images []core.Image, admin bool) *libraryv1.Item {
	b := libraryv1.Item_builder{
		Id:              new(it.ID.String()),
		LibraryId:       new(it.LibraryID.String()),
		Kind:            new(itemKinds[it.Kind]),
		Name:            &it.Name,
		SortName:        &it.SortName,
		OriginalTitle:   &it.OriginalTitle,
		Overview:        &it.Overview,
		Tagline:         &it.Tagline,
		ProductionYear:  new(int32(it.ProductionYear)),
		OfficialRating:  &it.OfficialRating,
		CommunityRating: &it.CommunityRating,
		CriticRating:    &it.CriticRating,
		Genres:          it.Genres,
		Tags:            it.Tags,
		Studios:         it.Studios,
		Artists:         it.Artists,
		AlbumArtists:    it.AlbumArtists,
		DateAdded:       timestamp(it.DateAdded),
		Images:          imagesToProto(images),
	}
	if it.CustomRating != "" {
		b.OfficialRating = &it.CustomRating
	}
	if admin {
		b.Path = &it.Path
	}
	if !it.ParentID.IsZero() {
		b.ParentId = new(it.ParentID.String())
	}
	b.IndexNumber, b.ParentIndexNumber, b.IndexNumberEnd = int32Ptr(it.IndexNumber), int32Ptr(it.ParentIndexNumber), int32Ptr(it.IndexNumberEnd)
	if it.PremiereDate != nil {
		b.PremiereDate = timestamppb.New(*it.PremiereDate)
	}
	if it.EndDate != nil {
		b.EndDate = timestamppb.New(*it.EndDate)
	}
	if it.Runtime > 0 {
		b.Runtime = durationpb.New(it.Runtime)
	}
	if len(it.ExternalIDs) > 0 {
		b.ExternalIds = make(map[string]string, len(it.ExternalIDs))
		for k, v := range it.ExternalIDs {
			b.ExternalIds[string(k)] = v
		}
	}
	if st, ok := seriesStatuses[it.SeriesStatus]; ok {
		b.SeriesStatus = &st
	}
	if it.Extra != "" {
		b.Extra = new(extraKinds[it.Extra])
		b.OwnerId = new(it.OwnerID.String())
	}
	return b.Build()
}

func mediaSourceToProto(ms *core.MediaSource, admin bool) *libraryv1.MediaSource {
	b := libraryv1.MediaSource_builder{
		Id:        new(ms.ID.String()),
		ItemId:    new(ms.ItemID.String()),
		Name:      &ms.Name,
		Container: &ms.Container,
		SizeBytes: &ms.Size,
		Bitrate:   &ms.Bitrate,
		ProbeTime: timestamp(ms.ProbedAt),
	}
	if admin {
		b.Path = &ms.Path
	}
	if ms.Duration > 0 {
		b.Duration = durationpb.New(ms.Duration)
	}
	for i := range ms.Streams {
		b.Streams = append(b.Streams, streamToProto(&ms.Streams[i], admin))
	}
	for _, c := range ms.Chapters {
		b.Chapters = append(b.Chapters, libraryv1.Chapter_builder{
			Start: durationpb.New(c.Start), Title: &c.Title, HasImage: new(c.ImagePath != ""),
		}.Build())
	}
	return b.Build()
}

func streamToProto(st *core.MediaStream, admin bool) *libraryv1.MediaStream {
	b := libraryv1.MediaStream_builder{
		Index:           new(int32(st.Index)),
		Kind:            new(streamKinds[st.Kind]),
		Codec:           &st.Codec,
		CodecTag:        &st.CodecTag,
		Profile:         &st.Profile,
		Level:           new(int32(st.Level)),
		Bitrate:         &st.Bitrate,
		Language:        &st.Language,
		Title:           &st.Title,
		Default:         &st.Default,
		Forced:          &st.Forced,
		HearingImpaired: &st.HearingImpaired,
	}
	if admin && st.ExternalPath != "" {
		b.ExternalPath = &st.ExternalPath
	}
	switch st.Kind {
	case core.StreamVideo:
		b.Width, b.Height = new(int32(st.Width)), new(int32(st.Height))
		b.FrameRate = rationalToProto(st.FrameRate)
		b.PixelFormat, b.BitDepth = &st.PixelFormat, new(int32(st.BitDepth))
		b.ColorRange, b.ColorPrimaries, b.ColorTransfer, b.ColorSpace = &st.ColorRange, &st.ColorPrimaries, &st.ColorTransfer, &st.ColorSpace
		r, ok := videoRanges[st.VideoRangeType()]
		if !ok && st.DolbyVision != nil {
			r, ok = libraryv1.VideoRange_VIDEO_RANGE_DOLBY_VISION, true
		}
		if ok {
			b.Range = &r
		}
		if dv := st.DolbyVision; dv != nil {
			b.DolbyVision = libraryv1.DolbyVision_builder{
				Profile: new(int32(dv.Profile)), Level: new(int32(dv.Level)), BlCompatibilityId: new(int32(dv.BLCompatibilityID)),
				RpuPresent: &dv.RPUPresent, ElPresent: &dv.ELPresent, BlPresent: &dv.BLPresent,
			}.Build()
		}
		b.Interlaced, b.Rotation = &st.Interlaced, new(int32(st.Rotation))
		b.SampleAspectRatio = rationalToProto(st.SampleAspectRatio)
	case core.StreamAudio:
		b.Channels, b.ChannelLayout, b.SampleRate = new(int32(st.Channels)), &st.ChannelLayout, new(int32(st.SampleRate))
	case core.StreamSubtitle:
		b.TextBased = new(st.IsTextSubtitle())
	}
	return b.Build()
}

func rationalToProto(r core.Rational) *libraryv1.Rational {
	if r.Den == 0 {
		return nil
	}
	return libraryv1.Rational_builder{Num: &r.Num, Den: &r.Den}.Build()
}

func personToProto(p *core.Person, images []core.Image) *libraryv1.Person {
	b := libraryv1.Person_builder{
		Id:         new(p.ID.String()),
		Name:       &p.Name,
		SortName:   &p.SortName,
		Overview:   &p.Overview,
		BirthPlace: &p.BirthPlace,
		Images:     imagesToProto(images),
	}
	if p.BirthDate != nil {
		b.BirthDate = timestamppb.New(*p.BirthDate)
	}
	if p.DeathDate != nil {
		b.DeathDate = timestamppb.New(*p.DeathDate)
	}
	if len(p.ExternalIDs) > 0 {
		b.ExternalIds = make(map[string]string, len(p.ExternalIDs))
		for k, v := range p.ExternalIDs {
			b.ExternalIds[string(k)] = v
		}
	}
	return b.Build()
}

// imagesToProto describes images without their file paths or source URLs;
// clients fetch them from the image endpoint by ID.
func imagesToProto(images []core.Image) []*libraryv1.Image {
	out := make([]*libraryv1.Image, len(images))
	for i := range images {
		img := &images[i]
		out[i] = libraryv1.Image_builder{
			Id:        new(img.ID.String()),
			Kind:      new(imageKinds[img.Kind]),
			Index:     new(int32(img.Index)),
			Width:     new(int32(img.Width)),
			Height:    new(int32(img.Height)),
			Blurhash:  &img.Blurhash,
			Thumbhash: img.Thumbhash,
		}.Build()
	}
	return out
}

func int32Ptr(p *int) *int32 {
	if p == nil {
		return nil
	}
	return new(int32(*p))
}

func timestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
