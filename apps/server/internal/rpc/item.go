package rpc

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

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
	sortFields = map[libraryv1.SortField]core.SortField{
		libraryv1.SortField_SORT_FIELD_NAME: core.SortName, libraryv1.SortField_SORT_FIELD_DATE_ADDED: core.SortDateAdded,
		libraryv1.SortField_SORT_FIELD_PREMIERE_DATE: core.SortPremiereDate, libraryv1.SortField_SORT_FIELD_PRODUCTION_YEAR: core.SortProductionYear,
		libraryv1.SortField_SORT_FIELD_COMMUNITY_RATING: core.SortCommunityRating, libraryv1.SortField_SORT_FIELD_RUNTIME: core.SortRuntime,
		libraryv1.SortField_SORT_FIELD_INDEX: core.SortIndex, libraryv1.SortField_SORT_FIELD_RANDOM: core.SortRandom,
		libraryv1.SortField_SORT_FIELD_LAST_PLAYED: core.SortLastPlayed, libraryv1.SortField_SORT_FIELD_PLAY_COUNT: core.SortPlayCount,
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
	store core.Store
}

var _ libraryv1connect.ItemServiceHandler = (*ItemService)(nil)

// NewItemService returns an ItemService backed by store.
func NewItemService(store core.Store) *ItemService { return &ItemService{store: store} }

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
	if !p.User.Policy.CanAccess(&item) {
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

	resp := libraryv1.GetItemResponse_builder{Item: itemToProto(&item, images[item.ID], p.User.Admin)}
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
	// Restrict to the requested libraries the user may access; a user
	// limited to libraries never sees others.
	ids := req.GetLibraryIds()
	if len(ids) == 0 && policy.Libraries != nil {
		if len(policy.Libraries) == 0 {
			return &libraryv1.ListItemsResponse{}, nil
		}
		q.LibraryIDs = policy.Libraries
	}
	for _, id := range ids {
		if lib := core.MustParseID(id); policy.CanAccessLibrary(lib) {
			q.LibraryIDs = append(q.LibraryIDs, lib)
		}
	}
	if len(ids) > 0 && len(q.LibraryIDs) == 0 {
		return &libraryv1.ListItemsResponse{}, nil
	}
	if req.HasParentId() {
		q.ParentID = core.MustParseID(req.GetParentId())
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
	for _, k := range req.GetKinds() {
		for ck, pk := range itemKinds {
			if pk == k {
				q.Kinds = append(q.Kinds, ck)
			}
		}
	}
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
	owners := make([]core.ID, len(page.Items))
	for i := range page.Items {
		owners[i] = page.Items[i].ID
	}
	images, err := s.store.Images().ListForOwners(ctx, owners)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*libraryv1.Item, len(page.Items))
	for i := range page.Items {
		out[i] = itemToProto(&page.Items[i], images[page.Items[i].ID], p.User.Admin)
	}
	return libraryv1.ListItemsResponse_builder{Items: out, Total: new(int32(page.Total))}.Build(), nil
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
