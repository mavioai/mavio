package app

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// fakeAcquisition serves what decides which releases a fake server grabs:
// its indexers and download clients, and its profiles, formats and rules.
type fakeAcquisition struct {
	acquisitionv1connect.UnimplementedProfileServiceHandler

	mu          sync.Mutex
	definitions map[acquisitionv1.MediaKind][]*acquisitionv1.QualityDefinition
	profiles    []*acquisitionv1.QualityProfile
	formats     []*acquisitionv1.CustomFormat
	delays      []*acquisitionv1.DelayProfile
	releases    []*acquisitionv1.ReleaseProfile
	mappings    []*acquisitionv1.RemotePathMapping
	ids         int
}

// fakeIndexers serves the indexers of a fake server's plugins.
type fakeIndexers struct {
	acquisitionv1connect.UnimplementedReleaseServiceHandler
}

// handleAcquisition adds the handlers of acquisition's settings to mux.
func (f *fakeCore) handleAcquisition(mux *http.ServeMux) {
	q := func(quality int32, title string, lo, hi float64) *acquisitionv1.QualityDefinition {
		return acquisitionv1.QualityDefinition_builder{Quality: &quality, Title: &title, MinSize: &lo, MaxSize: &hi, PreferredSize: new(hi / 2)}.Build()
	}
	item := func(quality int32, allowed bool) *acquisitionv1.QualityProfileItem {
		return acquisitionv1.QualityProfileItem_builder{Quality: &quality, Allowed: &allowed}.Build()
	}
	movie, series := acquisitionv1.MediaKind_MEDIA_KIND_MOVIE, acquisitionv1.MediaKind_MEDIA_KIND_SERIES
	f.acq = &fakeAcquisition{
		definitions: map[acquisitionv1.MediaKind][]*acquisitionv1.QualityDefinition{
			movie:  {q(3, "WEBDL-1080p", 2, 100), q(7, "Bluray-1080p", 4, 150), q(31, "Bluray-2160p", 10, 400)},
			series: {q(3, "WEBDL-1080p", 2, 80), q(19, "Bluray-2160p", 8, 300)},
		},
		profiles: []*acquisitionv1.QualityProfile{acquisitionv1.QualityProfile_builder{
			Id: new("hd"), Kind: &movie, Name: new("HD-1080p"), UpgradeAllowed: new(true), Cutoff: new(int32(7)),
			Items: []*acquisitionv1.QualityProfileItem{item(3, true), item(7, true), item(31, false)},
		}.Build()},
		formats: []*acquisitionv1.CustomFormat{acquisitionv1.CustomFormat_builder{
			Id: new("f-x265"), Name: new("x265"), Specifications: []*acquisitionv1.CustomFormatSpec{acquisitionv1.CustomFormatSpec_builder{
				Kind: new(acquisitionv1.CustomFormatSpecKind_CUSTOM_FORMAT_SPEC_KIND_RELEASE_TITLE), Name: new("x265"), Value: new(`\bx265\b`),
			}.Build()},
		}.Build()},
		delays: []*acquisitionv1.DelayProfile{acquisitionv1.DelayProfile_builder{
			Id: new("d-default"), Default: new(true), EnableUsenet: new(true), EnableTorrent: new(true),
			PreferredProtocol: new(acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_USENET), TorrentDelay: durationpb.New(30 * time.Minute),
		}.Build()},
		mappings: []*acquisitionv1.RemotePathMapping{acquisitionv1.RemotePathMapping_builder{
			Id: new("m1"), Host: new("seedbox"), RemotePath: new("/data/"), LocalPath: new("/mnt/seedbox/"),
		}.Build()},
	}
	mux.Handle(acquisitionv1connect.NewProfileServiceHandler(f.acq))
	mux.Handle(acquisitionv1connect.NewReleaseServiceHandler(fakeIndexers{}))
	f.downloads = newFakeDownloads()
	mux.Handle(acquisitionv1connect.NewDownloadServiceHandler(f.downloads))
	f.subtitles = newFakeSubtitles()
	mux.Handle(acquisitionv1connect.NewSubtitleServiceHandler(f.subtitles))
	f.ai = newFakeAI()
	mux.Handle(systemv1connect.NewAiProviderServiceHandler(f.ai))
	f.convs = newFakeConversations()
	mux.Handle(systemv1connect.NewAiConversationServiceHandler(f.convs))
	f.issues = newFakeIssues(f.record().GetUserId())
	mux.Handle(acquisitionv1connect.NewIssueServiceHandler(f.issues))
	f.wanted = newFakeWanted()
	mux.Handle(acquisitionv1connect.NewWantedServiceHandler(f.wanted))
}

func (fakeIndexers) SearchReleases(context.Context, *acquisitionv1.SearchReleasesRequest) (*acquisitionv1.SearchReleasesResponse, error) {
	torrent := acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT
	quality := func(name string) *acquisitionv1.ParsedRelease {
		return acquisitionv1.ParsedRelease_builder{Quality: acquisitionv1.ReleaseQuality_builder{Name: &name}.Build(), Languages: []string{"English"}}.Build()
	}
	return acquisitionv1.SearchReleasesResponse_builder{
		Releases: []*acquisitionv1.Release{
			acquisitionv1.Release_builder{
				IndexerId: new("1"), IndexerName: new("Nyaa"), Guid: new("g1"), Title: new("Orbit.2024.1080p.BluRay.x265-GRP"), Size: new(int64(6_400_000_000)),
				Protocol: &torrent, Seeders: new(int32(42)), Peers: new(int32(50)), Parsed: quality("Bluray-1080p"),
				CustomFormats: []string{"x265"}, CustomFormatScore: new(int32(10)), PublishDate: timestamppb.New(time.Now().Add(-72 * time.Hour)),
				Categories: []int32{2040, 2000}, Grabs: new(int32(12)), MagnetUrl: new("magnet:?xt=urn:btih:orbit"), InfoUrl: new("https://nyaa.example/view/1"),
			}.Build(),
			acquisitionv1.Release_builder{
				IndexerId: new("1"), IndexerName: new("Nyaa"), Guid: new("g2"), Title: new("Orbit.2024.CAM"), Size: new(int64(900_000_000)),
				Protocol: &torrent, Seeders: new(int32(3)), Peers: new(int32(4)), Parsed: quality("CAM"),
				Rejections: []*acquisitionv1.Rejection{acquisitionv1.Rejection_builder{Reason: new("quality_not_wanted"), Message: new("CAM is not wanted in profile")}.Build()},
			}.Build(),
		},
		Failures: []*acquisitionv1.IndexerFailure{acquisitionv1.IndexerFailure_builder{PluginId: new("dev.mavio.prowlarr"), IndexerId: new("2"), Message: new("timed out")}.Build()},
	}.Build(), nil
}

func (fakeIndexers) ListIndexers(context.Context, *acquisitionv1.ListIndexersRequest) (*acquisitionv1.ListIndexersResponse, error) {
	return acquisitionv1.ListIndexersResponse_builder{
		Indexers: []*acquisitionv1.Indexer{
			acquisitionv1.Indexer_builder{
				PluginId: new("dev.mavio.tmdb"), Id: new("1"), Name: new("Nyaa"), Enabled: new(true), Priority: new(int32(25)),
				Protocol: new(acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT), Privacy: new(acquisitionv1.IndexerPrivacy_INDEXER_PRIVACY_PUBLIC),
			}.Build(),
			acquisitionv1.Indexer_builder{
				PluginId: new("dev.mavio.tmdb"), Id: new("2"), Name: new("Broken"), Enabled: new(true), Priority: new(int32(25)),
				Protocol: new(acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT), Failure: new("timed out"),
				DisabledUntil: timestamppb.New(time.Now().Add(3 * time.Hour)),
			}.Build(),
		},
		Failures: []*acquisitionv1.IndexerFailure{acquisitionv1.IndexerFailure_builder{PluginId: new("dev.mavio.prowlarr"), Message: new("not reached")}.Build()},
	}.Build(), nil
}

func (s *fakeAcquisition) nextID(prefix string) string {
	s.ids++
	return fmt.Sprintf("%s%d", prefix, s.ids)
}

func (s *fakeAcquisition) ListQualityDefinitions(_ context.Context, r *acquisitionv1.ListQualityDefinitionsRequest) (*acquisitionv1.ListQualityDefinitionsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return acquisitionv1.ListQualityDefinitionsResponse_builder{Definitions: slices.Clone(s.definitions[r.GetKind()])}.Build(), nil
}

func (s *fakeAcquisition) UpdateQualityDefinitions(_ context.Context, r *acquisitionv1.UpdateQualityDefinitionsRequest) (*acquisitionv1.UpdateQualityDefinitionsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := slices.Clone(s.definitions[r.GetKind()])
	for _, d := range r.GetDefinitions() {
		if i := slices.IndexFunc(list, func(x *acquisitionv1.QualityDefinition) bool { return x.GetQuality() == d.GetQuality() }); i >= 0 {
			list[i] = d
		}
	}
	s.definitions[r.GetKind()] = list
	return acquisitionv1.UpdateQualityDefinitionsResponse_builder{Definitions: list}.Build(), nil
}

func (s *fakeAcquisition) ListQualityProfiles(context.Context, *acquisitionv1.ListQualityProfilesRequest) (*acquisitionv1.ListQualityProfilesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return acquisitionv1.ListQualityProfilesResponse_builder{Profiles: slices.Clone(s.profiles)}.Build(), nil
}

func (s *fakeAcquisition) CreateQualityProfile(_ context.Context, r *acquisitionv1.CreateQualityProfileRequest) (*acquisitionv1.CreateQualityProfileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := proto.CloneOf(r.GetProfile())
	p.SetId(s.nextID("qp"))
	s.profiles = append(s.profiles, p)
	return acquisitionv1.CreateQualityProfileResponse_builder{Profile: p}.Build(), nil
}

func (s *fakeAcquisition) UpdateQualityProfile(_ context.Context, r *acquisitionv1.UpdateQualityProfileRequest) (*acquisitionv1.UpdateQualityProfileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := r.GetProfile()
	if !replaceByID(s.profiles, p, (*acquisitionv1.QualityProfile).GetId) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no profile %q", p.GetId()))
	}
	return acquisitionv1.UpdateQualityProfileResponse_builder{Profile: p}.Build(), nil
}

func (s *fakeAcquisition) DeleteQualityProfile(_ context.Context, r *acquisitionv1.DeleteQualityProfileRequest) (*acquisitionv1.DeleteQualityProfileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profiles = deleteByID(s.profiles, r.GetId(), (*acquisitionv1.QualityProfile).GetId)
	return &acquisitionv1.DeleteQualityProfileResponse{}, nil
}

func (s *fakeAcquisition) ListCustomFormats(context.Context, *acquisitionv1.ListCustomFormatsRequest) (*acquisitionv1.ListCustomFormatsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return acquisitionv1.ListCustomFormatsResponse_builder{Formats: slices.Clone(s.formats)}.Build(), nil
}

func (s *fakeAcquisition) CreateCustomFormat(_ context.Context, r *acquisitionv1.CreateCustomFormatRequest) (*acquisitionv1.CreateCustomFormatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := proto.CloneOf(r.GetFormat())
	f.SetId(s.nextID("f"))
	s.formats = append(s.formats, f)
	return acquisitionv1.CreateCustomFormatResponse_builder{Format: f}.Build(), nil
}

func (s *fakeAcquisition) UpdateCustomFormat(_ context.Context, r *acquisitionv1.UpdateCustomFormatRequest) (*acquisitionv1.UpdateCustomFormatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	replaceByID(s.formats, r.GetFormat(), (*acquisitionv1.CustomFormat).GetId)
	return acquisitionv1.UpdateCustomFormatResponse_builder{Format: r.GetFormat()}.Build(), nil
}

func (s *fakeAcquisition) DeleteCustomFormat(_ context.Context, r *acquisitionv1.DeleteCustomFormatRequest) (*acquisitionv1.DeleteCustomFormatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.formats = deleteByID(s.formats, r.GetId(), (*acquisitionv1.CustomFormat).GetId)
	return &acquisitionv1.DeleteCustomFormatResponse{}, nil
}

func (s *fakeAcquisition) ListDelayProfiles(context.Context, *acquisitionv1.ListDelayProfilesRequest) (*acquisitionv1.ListDelayProfilesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return acquisitionv1.ListDelayProfilesResponse_builder{Profiles: slices.Clone(s.delays)}.Build(), nil
}

func (s *fakeAcquisition) CreateDelayProfile(_ context.Context, r *acquisitionv1.CreateDelayProfileRequest) (*acquisitionv1.CreateDelayProfileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := proto.CloneOf(r.GetProfile())
	p.SetId(s.nextID("d"))
	// Before the default one, which stays last.
	s.delays = slices.Insert(s.delays, len(s.delays)-1, p)
	return acquisitionv1.CreateDelayProfileResponse_builder{Profile: p}.Build(), nil
}

func (s *fakeAcquisition) UpdateDelayProfile(_ context.Context, r *acquisitionv1.UpdateDelayProfileRequest) (*acquisitionv1.UpdateDelayProfileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	replaceByID(s.delays, r.GetProfile(), (*acquisitionv1.DelayProfile).GetId)
	return acquisitionv1.UpdateDelayProfileResponse_builder{Profile: r.GetProfile()}.Build(), nil
}

func (s *fakeAcquisition) DeleteDelayProfile(_ context.Context, r *acquisitionv1.DeleteDelayProfileRequest) (*acquisitionv1.DeleteDelayProfileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delays = deleteByID(s.delays, r.GetId(), (*acquisitionv1.DelayProfile).GetId)
	return &acquisitionv1.DeleteDelayProfileResponse{}, nil
}

func (s *fakeAcquisition) ReorderDelayProfiles(_ context.Context, r *acquisitionv1.ReorderDelayProfilesRequest) (*acquisitionv1.ReorderDelayProfilesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	slices.SortStableFunc(s.delays, func(x, y *acquisitionv1.DelayProfile) int {
		rank := func(p *acquisitionv1.DelayProfile) int {
			if p.GetDefault() {
				return len(r.GetIds())
			}
			return slices.Index(r.GetIds(), p.GetId())
		}
		return rank(x) - rank(y)
	})
	return acquisitionv1.ReorderDelayProfilesResponse_builder{Profiles: slices.Clone(s.delays)}.Build(), nil
}

func (s *fakeAcquisition) ListReleaseProfiles(context.Context, *acquisitionv1.ListReleaseProfilesRequest) (*acquisitionv1.ListReleaseProfilesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return acquisitionv1.ListReleaseProfilesResponse_builder{Profiles: slices.Clone(s.releases)}.Build(), nil
}

func (s *fakeAcquisition) CreateReleaseProfile(_ context.Context, r *acquisitionv1.CreateReleaseProfileRequest) (*acquisitionv1.CreateReleaseProfileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := proto.CloneOf(r.GetProfile())
	p.SetId(s.nextID("r"))
	s.releases = append(s.releases, p)
	return acquisitionv1.CreateReleaseProfileResponse_builder{Profile: p}.Build(), nil
}

func (s *fakeAcquisition) UpdateReleaseProfile(_ context.Context, r *acquisitionv1.UpdateReleaseProfileRequest) (*acquisitionv1.UpdateReleaseProfileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	replaceByID(s.releases, r.GetProfile(), (*acquisitionv1.ReleaseProfile).GetId)
	return acquisitionv1.UpdateReleaseProfileResponse_builder{Profile: r.GetProfile()}.Build(), nil
}

func (s *fakeAcquisition) DeleteReleaseProfile(_ context.Context, r *acquisitionv1.DeleteReleaseProfileRequest) (*acquisitionv1.DeleteReleaseProfileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releases = deleteByID(s.releases, r.GetId(), (*acquisitionv1.ReleaseProfile).GetId)
	return &acquisitionv1.DeleteReleaseProfileResponse{}, nil
}

func (s *fakeAcquisition) ListRemotePathMappings(context.Context, *acquisitionv1.ListRemotePathMappingsRequest) (*acquisitionv1.ListRemotePathMappingsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return acquisitionv1.ListRemotePathMappingsResponse_builder{Mappings: slices.Clone(s.mappings)}.Build(), nil
}

func (s *fakeAcquisition) CreateRemotePathMapping(_ context.Context, r *acquisitionv1.CreateRemotePathMappingRequest) (*acquisitionv1.CreateRemotePathMappingResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := proto.CloneOf(r.GetMapping())
	m.SetId(s.nextID("m"))
	s.mappings = append(s.mappings, m)
	return acquisitionv1.CreateRemotePathMappingResponse_builder{Mapping: m}.Build(), nil
}

func (s *fakeAcquisition) DeleteRemotePathMapping(_ context.Context, r *acquisitionv1.DeleteRemotePathMappingRequest) (*acquisitionv1.DeleteRemotePathMappingResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mappings = deleteByID(s.mappings, r.GetId(), (*acquisitionv1.RemotePathMapping).GetId)
	return &acquisitionv1.DeleteRemotePathMappingResponse{}, nil
}

// replaceByID replaces the entry of list with v's ID by v, and reports
// whether there was one.
func replaceByID[T any](list []T, v T, id func(T) string) bool {
	i := slices.IndexFunc(list, func(x T) bool { return id(x) == id(v) })
	if i >= 0 {
		list[i] = v
	}
	return i >= 0
}

// deleteByID returns list without the entry with the ID.
func deleteByID[T any](list []T, want string, id func(T) string) []T {
	return slices.DeleteFunc(list, func(x T) bool { return id(x) == want })
}

// fakeWanted serves a fake server's wanted movies and episodes, whose
// searches it counts.
type fakeWanted struct {
	acquisitionv1connect.UnimplementedWantedServiceHandler

	mu        sync.Mutex
	movies    []*acquisitionv1.WantedMovie
	episodes  []*acquisitionv1.WantedEpisode
	searched  []string // the items searched for, "all" for all
	rssSynced bool
	updated   []*acquisitionv1.Wanted
}

func newFakeWanted() *fakeWanted {
	wanted := func(id string) *acquisitionv1.Wanted {
		return acquisitionv1.Wanted_builder{ItemId: &id, Monitored: new(true)}.Build()
	}
	return &fakeWanted{
		movies: []*acquisitionv1.WantedMovie{
			acquisitionv1.WantedMovie_builder{ItemId: new("dune"), Name: new("Dune"), Year: new(int32(2021)), Available: new(true), Wanted: wanted("dune")}.Build(),
			acquisitionv1.WantedMovie_builder{ItemId: new("arrival"), Name: new("Arrival"), Year: new(int32(2016)), Wanted: wanted("arrival")}.Build(),
		},
		episodes: []*acquisitionv1.WantedEpisode{acquisitionv1.WantedEpisode_builder{
			ItemId: new("e1"), SeriesName: new("Severance"), SeasonNumber: new(int32(2)), EpisodeNumber: new(int32(3)), Name: new("Who Is Alive?"),
			Quality: new("WEBDL-720p"),
		}.Build()},
	}
}

func (s *fakeWanted) ListWanted(_ context.Context, r *acquisitionv1.ListWantedRequest) (*acquisitionv1.ListWantedResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.GetKind() == acquisitionv1.WantedKind_WANTED_KIND_EPISODE {
		return acquisitionv1.ListWantedResponse_builder{Episodes: slices.Clone(s.episodes), Total: new(int32(len(s.episodes)))}.Build(), nil
	}
	return acquisitionv1.ListWantedResponse_builder{Movies: slices.Clone(s.movies), Total: new(int32(len(s.movies)))}.Build(), nil
}

func (s *fakeWanted) SearchWanted(_ context.Context, r *acquisitionv1.SearchWantedRequest) (*acquisitionv1.SearchWantedResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(r.GetItemIds()) == 0 {
		s.searched = append(s.searched, "all")
		return acquisitionv1.SearchWantedResponse_builder{Queued: new(int32(len(s.movies)))}.Build(), nil
	}
	s.searched = append(s.searched, r.GetItemIds()...)
	return acquisitionv1.SearchWantedResponse_builder{Queued: new(int32(len(r.GetItemIds())))}.Build(), nil
}

func (s *fakeWanted) SyncRSS(context.Context, *acquisitionv1.SyncRSSRequest) (*acquisitionv1.SyncRSSResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rssSynced = true
	return &acquisitionv1.SyncRSSResponse{}, nil
}

func (s *fakeWanted) SetMonitored(_ context.Context, r *acquisitionv1.SetMonitoredRequest) (*acquisitionv1.SetMonitoredResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !r.GetItems().GetMonitored() {
		s.episodes = slices.DeleteFunc(s.episodes, func(e *acquisitionv1.WantedEpisode) bool {
			return slices.Contains(r.GetItems().GetItemIds(), e.GetItemId())
		})
	}
	return &acquisitionv1.SetMonitoredResponse{}, nil
}

func (s *fakeWanted) GetWanted(_ context.Context, r *acquisitionv1.GetWantedRequest) (*acquisitionv1.GetWantedResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.movies {
		if m.GetItemId() == r.GetItemId() {
			return acquisitionv1.GetWantedResponse_builder{Wanted: m.GetWanted()}.Build(), nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%q is not wanted", r.GetItemId()))
}

func (s *fakeWanted) UpdateWanted(_ context.Context, r *acquisitionv1.UpdateWantedRequest) (*acquisitionv1.UpdateWantedResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updated = append(s.updated, r.GetWanted())
	if !r.GetWanted().GetMonitored() {
		s.movies = slices.DeleteFunc(s.movies, func(m *acquisitionv1.WantedMovie) bool { return m.GetItemId() == r.GetWanted().GetItemId() })
	}
	return acquisitionv1.UpdateWantedResponse_builder{Wanted: r.GetWanted()}.Build(), nil
}
