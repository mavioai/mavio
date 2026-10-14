package rpc

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
)

var libraryKinds = map[core.LibraryKind]libraryv1.LibraryKind{
	core.LibraryMovies:      libraryv1.LibraryKind_LIBRARY_KIND_MOVIES,
	core.LibraryShows:       libraryv1.LibraryKind_LIBRARY_KIND_SHOWS,
	core.LibraryMusic:       libraryv1.LibraryKind_LIBRARY_KIND_MUSIC,
	core.LibraryMusicVideos: libraryv1.LibraryKind_LIBRARY_KIND_MUSIC_VIDEOS,
	core.LibraryHomeVideos:  libraryv1.LibraryKind_LIBRARY_KIND_HOME_VIDEOS,
	core.LibraryBooks:       libraryv1.LibraryKind_LIBRARY_KIND_BOOKS,
	core.LibraryPhotos:      libraryv1.LibraryKind_LIBRARY_KIND_PHOTOS,
	core.LibraryMixed:       libraryv1.LibraryKind_LIBRARY_KIND_MIXED,
	core.LibraryCollections: libraryv1.LibraryKind_LIBRARY_KIND_COLLECTIONS,
	core.LibraryPlaylists:   libraryv1.LibraryKind_LIBRARY_KIND_PLAYLISTS,
}

// errCurated rejects managing the curated libraries, which the server
// keeps.
var errCurated = errors.New("the collections and playlists libraries are managed by the server")

// LibraryService implements mavio.library.v1.LibraryService. Users see the
// libraries their policy allows; only administrators see folder paths and
// manage libraries.
type LibraryService struct {
	// Classes returns the storage class of each path of a library; nil
	// reports the classes the library sets.
	Classes func(lib *core.Library) map[string]core.StorageClass
	// Storages opens the storage of library paths, which must be folders;
	// nil opens local folders only.
	Storages *library.Storages

	store core.Store
	now   func() time.Time
}

var _ libraryv1connect.LibraryServiceHandler = (*LibraryService)(nil)

// NewLibraryService returns a LibraryService backed by store.
func NewLibraryService(store core.Store) *LibraryService {
	return &LibraryService{store: store, now: time.Now}
}

// ListLibraries lists the libraries the caller may access.
func (s *LibraryService) ListLibraries(ctx context.Context, _ *libraryv1.ListLibrariesRequest) (*libraryv1.ListLibrariesResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	libs, err := s.store.Libraries().List(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	var out []*libraryv1.Library
	for i := range libs {
		if canAccessLibrary(&p.User, &libs[i]) {
			out = append(out, s.toProto(ctx, &libs[i], p.User.Admin))
		}
	}
	return libraryv1.ListLibrariesResponse_builder{Libraries: out}.Build(), nil
}

// GetLibrary returns a library the caller may access.
func (s *LibraryService) GetLibrary(ctx context.Context, req *libraryv1.GetLibraryRequest) (*libraryv1.GetLibraryResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	lib, err := s.store.Libraries().Get(ctx, core.MustParseID(req.GetId()))
	if errors.Is(err, core.ErrNotFound) || err == nil && !canAccessLibrary(&p.User, &lib) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such library"))
	}
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.GetLibraryResponse_builder{Library: s.toProto(ctx, &lib, p.User.Admin)}.Build(), nil
}

// canAccessLibrary reports whether the user may see a library: every user
// has the playlists library, where they find their own playlists.
func canAccessLibrary(u *core.User, lib *core.Library) bool {
	return lib.Kind == core.LibraryPlaylists || u.Policy.CanAccessLibrary(lib.ID)
}

// CreateLibrary creates a library and queues its first scan.
func (s *LibraryService) CreateLibrary(ctx context.Context, req *libraryv1.CreateLibraryRequest) (*libraryv1.CreateLibraryResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	lib, err := s.libraryFromSpec(ctx, req.GetSpec())
	if err != nil {
		return nil, err
	}
	err = s.store.InTx(ctx, func(tx core.Store) error {
		if err := tx.Libraries().Create(ctx, &lib); err != nil {
			return err
		}
		return s.queueScans(ctx, tx, &lib)
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.CreateLibraryResponse_builder{Library: s.toProto(ctx, &lib, true)}.Build(), nil
}

// UpdateLibrary changes a library and queues a scan of it.
func (s *LibraryService) UpdateLibrary(ctx context.Context, req *libraryv1.UpdateLibraryRequest) (*libraryv1.UpdateLibraryResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	spec, err := s.libraryFromSpec(ctx, req.GetSpec())
	if err != nil {
		return nil, err
	}
	var lib core.Library
	err = s.store.InTx(ctx, func(tx core.Store) error {
		if lib, err = tx.Libraries().Get(ctx, core.MustParseID(req.GetId())); err != nil {
			return err
		}
		if lib.Kind.Curated() {
			return connect.NewError(connect.CodeFailedPrecondition, errCurated)
		}
		lib.Name, lib.Kind, lib.Paths, lib.ScanInterval = spec.Name, spec.Kind, spec.Paths, spec.ScanInterval
		lib.PreferredLanguage, lib.MetadataCountry = spec.PreferredLanguage, spec.MetadataCountry
		lib.SaveLocalMetadata, lib.AutoCollections = spec.SaveLocalMetadata, spec.AutoCollections
		lib.ExtractTrickplay, lib.ExtractChapterImages, lib.AnalyzeLoudness = spec.ExtractTrickplay, spec.ExtractChapterImages, spec.AnalyzeLoudness
		lib.DownloadLyrics, lib.Providers, lib.VirtualEpisodes = spec.DownloadLyrics, spec.Providers, spec.VirtualEpisodes
		lib.StorageClasses = spec.StorageClasses
		if err := tx.Libraries().Update(ctx, &lib); err != nil {
			return err
		}
		return s.queueScans(ctx, tx, &lib)
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.UpdateLibraryResponse_builder{Library: s.toProto(ctx, &lib, true)}.Build(), nil
}

// queueScans queues a scan of the library now and, with a scan interval,
// the scheduled scan after it.
func (s *LibraryService) queueScans(ctx context.Context, tx core.Store, lib *core.Library) error {
	now := s.now()
	jobs := []core.Job{library.ScanJob(*lib, now, false)}
	if lib.ScanInterval > 0 {
		jobs = append(jobs, library.ScanJob(*lib, now.Add(lib.ScanInterval), true))
	}
	for i := range jobs {
		if _, err := tx.Jobs().Enqueue(ctx, &jobs[i]); err != nil {
			return err
		}
	}
	return nil
}

// DeleteLibrary removes a library and its items; media files stay.
func (s *LibraryService) DeleteLibrary(ctx context.Context, req *libraryv1.DeleteLibraryRequest) (*libraryv1.DeleteLibraryResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if err := s.store.Libraries().Delete(ctx, core.MustParseID(req.GetId())); err != nil {
		return nil, connectError(ctx, err)
	}
	return &libraryv1.DeleteLibraryResponse{}, nil
}

// ScanLibrary queues a scan unless one is pending; one asked for while a
// scan runs runs after it.
func (s *LibraryService) ScanLibrary(ctx context.Context, req *libraryv1.ScanLibraryRequest) (*libraryv1.ScanLibraryResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	lib, err := s.store.Libraries().Get(ctx, core.MustParseID(req.GetId()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if lib.Kind.Curated() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errCurated)
	}
	job := library.ScanJob(lib, s.now(), false)
	added, err := s.store.Jobs().Enqueue(ctx, &job)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	b := libraryv1.ScanLibraryResponse_builder{Enqueued: &added}
	if added {
		b.JobId = new(job.ID.String())
	}
	return b.Build(), nil
}

// libraryFromSpec converts a validated spec, checking that its paths are
// absolute folders or folders of rclone remotes.
func (s *LibraryService) libraryFromSpec(ctx context.Context, spec *libraryv1.LibrarySpec) (core.Library, error) {
	lib := core.Library{
		Name: spec.GetName(), Paths: spec.GetPaths(),
		PreferredLanguage: spec.GetPreferredLanguage(), MetadataCountry: spec.GetMetadataCountry(),
		SaveLocalMetadata: spec.GetSaveLocalMetadata(), AutoCollections: spec.GetAutoCollections(),
		ExtractTrickplay: spec.GetExtractTrickplay(), ExtractChapterImages: spec.GetExtractChapterImages(),
		AnalyzeLoudness: spec.GetAnalyzeLoudness(), DownloadLyrics: spec.GetDownloadLyrics(),
		Providers: providerOrderFromProto(spec.GetProviderOrder()),
		Hubs:      libraryHubsFromProto(spec.GetHubs()),
	}
	if v := spec.GetVirtualEpisodes(); v != nil {
		lib.VirtualEpisodes = core.VirtualEpisodes{
			Missing: v.GetMissing(), Unaired: v.GetUnaired(), Specials: v.GetSpecials(), GraceDays: int(v.GetGraceDays()),
		}
	}
	for k, v := range libraryKinds {
		if v == spec.GetKind() {
			lib.Kind = k
		}
	}
	for p, c := range spec.GetStorageClasses() {
		if lib.StorageClasses == nil {
			lib.StorageClasses = map[string]core.StorageClass{}
		}
		lib.StorageClasses[p] = storageClassFromProto(c)
	}
	if lib.Kind.Curated() {
		return lib, connect.NewError(connect.CodeInvalidArgument, errCurated)
	}
	if spec.HasScanInterval() {
		lib.ScanInterval = spec.GetScanInterval().AsDuration()
	}
	for _, p := range lib.Paths {
		if !core.IsRemotePath(p) && !filepath.IsAbs(p) {
			return lib, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("library path %q is not absolute", p))
		}
		if !s.isFolder(ctx, p) {
			return lib, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("library path %q is not a folder", p))
		}
	}
	return lib, nil
}

// isFolder reports whether a library path is a folder of its storage.
func (s *LibraryService) isFolder(ctx context.Context, p string) bool {
	st, err := s.Storages.Open(ctx, p)
	if err != nil {
		return false
	}
	defer st.Close()
	info, err := st.Stat(ctx, ".")
	return err == nil && info.IsDir()
}

// toProto converts a library; folder paths and their storage classes only
// for administrators.
func (s *LibraryService) toProto(ctx context.Context, lib *core.Library, admin bool) *libraryv1.Library {
	l := libraryToProto(lib, admin)
	l.SetImageUrl(imageURL(ctx, "images/collages/"+lib.ID.String(), 3))
	if admin && len(lib.Paths) > 0 {
		classes := lib.StorageClasses
		if s.Classes != nil {
			classes = s.Classes(lib)
		}
		l.SetEffectiveStorageClasses(storageClassesToProto(classes))
	}
	return l
}

// storageClasses maps the storage classes to their protobuf values.
var storageClasses = map[core.StorageClass]libraryv1.StorageClass{
	core.StorageAuto:    libraryv1.StorageClass_STORAGE_CLASS_UNSPECIFIED,
	core.StorageLocal:   libraryv1.StorageClass_STORAGE_CLASS_LOCAL,
	core.StorageNetwork: libraryv1.StorageClass_STORAGE_CLASS_NETWORK,
	core.StorageCloud:   libraryv1.StorageClass_STORAGE_CLASS_CLOUD,
}

func storageClassFromProto(c libraryv1.StorageClass) core.StorageClass {
	for k, v := range storageClasses {
		if v == c {
			return k
		}
	}
	return core.StorageAuto
}

func storageClassesToProto(classes map[string]core.StorageClass) map[string]libraryv1.StorageClass {
	if len(classes) == 0 {
		return nil
	}
	out := make(map[string]libraryv1.StorageClass, len(classes))
	for p, c := range classes {
		out[p] = storageClasses[c]
	}
	return out
}

// libraryToProto converts a library; folder paths only for administrators.
func libraryToProto(lib *core.Library, admin bool) *libraryv1.Library {
	b := libraryv1.Library_builder{
		Id:                new(lib.ID.String()),
		Name:              &lib.Name,
		Kind:              new(libraryKinds[lib.Kind]),
		PreferredLanguage: &lib.PreferredLanguage,
		MetadataCountry:   &lib.MetadataCountry,
		SaveLocalMetadata: &lib.SaveLocalMetadata,
		AutoCollections:   &lib.AutoCollections,

		ExtractTrickplay:     &lib.ExtractTrickplay,
		ExtractChapterImages: &lib.ExtractChapterImages,
		AnalyzeLoudness:      &lib.AnalyzeLoudness,
		DownloadLyrics:       &lib.DownloadLyrics,
		ProviderOrder:        providerOrderToProto(&lib.Providers),
		Hubs:                 libraryHubsToProto(&lib.Hubs),
		VirtualEpisodes: libraryv1.VirtualEpisodes_builder{
			Missing: &lib.VirtualEpisodes.Missing, Unaired: &lib.VirtualEpisodes.Unaired, Specials: &lib.VirtualEpisodes.Specials,
			GraceDays: new(int32(lib.VirtualEpisodes.GraceDays)),
		}.Build(),
		CreateTime: timestamppb.New(lib.CreatedAt),
		UpdateTime: timestamppb.New(lib.UpdatedAt),
	}
	if admin {
		b.Paths = lib.Paths
		b.StorageClasses = storageClassesToProto(lib.StorageClasses)
	}
	if lib.ScanInterval > 0 {
		b.ScanInterval = durationpb.New(lib.ScanInterval)
	}
	return b.Build()
}

// providerOrderFromProto converts a library's provider order; unset lists
// stay nil.
func providerOrderFromProto(o *libraryv1.ProviderOrder) core.ProviderOrder {
	list := func(l *libraryv1.ProviderList) []string {
		if l == nil {
			return nil
		}
		return append([]string{}, l.GetPluginIds()...)
	}
	return core.ProviderOrder{
		Metadata: list(o.GetMetadata()), Images: list(o.GetImages()), Subtitles: list(o.GetSubtitles()),
		Lyrics: list(o.GetLyrics()), Segments: list(o.GetSegments()), Episodes: list(o.GetEpisodes()),
	}
}

// providerOrderToProto converts a library's provider order; nil lists stay
// unset.
func providerOrderToProto(o *core.ProviderOrder) *libraryv1.ProviderOrder {
	list := func(ids []string) *libraryv1.ProviderList {
		if ids == nil {
			return nil
		}
		return libraryv1.ProviderList_builder{PluginIds: ids}.Build()
	}
	return libraryv1.ProviderOrder_builder{
		Metadata: list(o.Metadata), Images: list(o.Images), Subtitles: list(o.Subtitles),
		Lyrics: list(o.Lyrics), Segments: list(o.Segments), Episodes: list(o.Episodes),
	}.Build()
}

// hubKinds maps the hub kinds to their protobuf values.
var hubKinds = map[core.HubKind]libraryv1.HubKind{
	core.HubContinueWatching:  libraryv1.HubKind_HUB_KIND_CONTINUE_WATCHING,
	core.HubRecentlyAdded:     libraryv1.HubKind_HUB_KIND_RECENTLY_ADDED,
	core.HubRecentlyReleased:  libraryv1.HubKind_HUB_KIND_RECENTLY_RELEASED,
	core.HubRecentlyAired:     libraryv1.HubKind_HUB_KIND_RECENTLY_AIRED,
	core.HubTopRated:          libraryv1.HubKind_HUB_KIND_TOP_RATED,
	core.HubTopUnplayed:       libraryv1.HubKind_HUB_KIND_TOP_UNPLAYED,
	core.HubStartWatching:     libraryv1.HubKind_HUB_KIND_START_WATCHING,
	core.HubRediscover:        libraryv1.HubKind_HUB_KIND_REDISCOVER,
	core.HubBecauseYouWatched: libraryv1.HubKind_HUB_KIND_BECAUSE_YOU_WATCHED,
	core.HubBecauseYouLike:    libraryv1.HubKind_HUB_KIND_BECAUSE_YOU_LIKE,
	core.HubMoreInGenre:       libraryv1.HubKind_HUB_KIND_MORE_IN_GENRE,
	core.HubStarring:          libraryv1.HubKind_HUB_KIND_STARRING,
	core.HubDirectedBy:        libraryv1.HubKind_HUB_KIND_DIRECTED_BY,
	core.HubAvailableRequests: libraryv1.HubKind_HUB_KIND_AVAILABLE_REQUESTS,
	core.HubTrending:          libraryv1.HubKind_HUB_KIND_TRENDING,
}

// libraryHubsFromProto converts a library's hubs; unset lists stay nil.
func libraryHubsFromProto(h *libraryv1.LibraryHubs) core.LibraryHubs {
	list := func(l *libraryv1.HubList) []core.HubKind {
		if l == nil {
			return nil
		}
		out := make([]core.HubKind, len(l.GetKinds()))
		for i, k := range l.GetKinds() {
			out[i] = keyOf(hubKinds, k)
		}
		return out
	}
	return core.LibraryHubs{Home: list(h.GetHome()), Recommended: list(h.GetRecommended())}
}

// libraryHubsToProto converts a library's hubs; nil lists stay unset.
func libraryHubsToProto(h *core.LibraryHubs) *libraryv1.LibraryHubs {
	list := func(kinds []core.HubKind) *libraryv1.HubList {
		if kinds == nil {
			return nil
		}
		out := make([]libraryv1.HubKind, len(kinds))
		for i, k := range kinds {
			out[i] = hubKinds[k]
		}
		return libraryv1.HubList_builder{Kinds: out}.Build()
	}
	return libraryv1.LibraryHubs_builder{Home: list(h.Home), Recommended: list(h.Recommended)}.Build()
}
