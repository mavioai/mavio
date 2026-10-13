package rpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/settings"
	"github.com/mavioai/mavio/apps/server/internal/subtitles"
	"github.com/mavioai/mavio/libs/core"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/subtitle"
)

// SubtitleService implements mavio.acquisition.v1.SubtitleService.
type SubtitleService struct {
	store core.Store
	rt    *subtitles.Runtime
	// Settings keeps the subtitle settings among the server's; nil
	// serves the defaults and changes none.
	Settings *settings.Manager
	// Activity records settings changes; nil records none.
	Activity interface {
		Record(ctx context.Context, a core.Activity)
	}
}

var _ acquisitionv1connect.SubtitleServiceHandler = (*SubtitleService)(nil)

// NewSubtitleService returns a SubtitleService acquiring subtitles with
// rt.
func NewSubtitleService(store core.Store, rt *subtitles.Runtime) *SubtitleService {
	return &SubtitleService{store: store, rt: rt}
}

// subtitleError maps the runtime's errors to Connect codes.
func subtitleError(ctx context.Context, err error) error {
	if errors.Is(err, subtitles.ErrUnavailable) {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connectError(ctx, err)
}

func languageFromProto(l *acquisitionv1.SubtitleLanguageSpec) core.SubtitleLanguage {
	return core.SubtitleLanguage{Language: l.GetLanguage(), Forced: l.GetForced(), HearingImpaired: l.GetHearingImpaired()}
}

func languageToProto(l core.SubtitleLanguage) *acquisitionv1.SubtitleLanguageSpec {
	return acquisitionv1.SubtitleLanguageSpec_builder{Language: new(l.Language), Forced: new(l.Forced), HearingImpaired: new(l.HearingImpaired)}.Build()
}

func languagesToProto(ls []core.SubtitleLanguage) []*acquisitionv1.SubtitleLanguageSpec {
	out := make([]*acquisitionv1.SubtitleLanguageSpec, len(ls))
	for i, l := range ls {
		out[i] = languageToProto(l)
	}
	return out
}

var subtitleHIs = map[acquisitionv1.SubtitleHearingImpaired]core.SubtitleHI{
	acquisitionv1.SubtitleHearingImpaired_SUBTITLE_HEARING_IMPAIRED_UNSPECIFIED: core.SubtitleHIAllowed,
	acquisitionv1.SubtitleHearingImpaired_SUBTITLE_HEARING_IMPAIRED_REQUIRED:    core.SubtitleHIRequired,
	acquisitionv1.SubtitleHearingImpaired_SUBTITLE_HEARING_IMPAIRED_EXCLUDED:    core.SubtitleHIExcluded,
}

func subtitleProfileFromProto(p *acquisitionv1.SubtitleProfile) core.SubtitleProfile {
	out := core.SubtitleProfile{
		Name: p.GetName(), Cutoff: int(p.GetCutoff()), MustContain: p.GetMustContain(), MustNotContain: p.GetMustNotContain(),
		OriginalFormat: p.GetOriginalFormat(), Tag: p.GetTag(),
	}
	for _, it := range p.GetItems() {
		out.Items = append(out.Items, core.SubtitleProfileItem{
			ID: int(it.GetId()), Language: it.GetLanguage(), Forced: it.GetForced(), HI: subtitleHIs[it.GetHearingImpaired()],
			AudioExclude: it.GetAudioExclude(), AudioOnlyInclude: it.GetAudioOnlyInclude(),
		})
	}
	return out
}

func subtitleProfileToProto(p *core.SubtitleProfile) *acquisitionv1.SubtitleProfile {
	if p == nil {
		return nil
	}
	items := make([]*acquisitionv1.SubtitleProfileItem, len(p.Items))
	for i, it := range p.Items {
		hi := acquisitionv1.SubtitleHearingImpaired_SUBTITLE_HEARING_IMPAIRED_UNSPECIFIED
		for k, v := range subtitleHIs {
			if v == it.HI {
				hi = k
			}
		}
		items[i] = acquisitionv1.SubtitleProfileItem_builder{
			Id: new(int32(it.ID)), Language: new(it.Language), Forced: new(it.Forced), HearingImpaired: hi.Enum(),
			AudioExclude: new(it.AudioExclude), AudioOnlyInclude: new(it.AudioOnlyInclude),
		}.Build()
	}
	return acquisitionv1.SubtitleProfile_builder{
		Id: new(p.ID.String()), Name: new(p.Name), Items: items, Cutoff: new(int32(p.Cutoff)),
		MustContain: p.MustContain, MustNotContain: p.MustNotContain, OriginalFormat: new(p.OriginalFormat), Tag: new(p.Tag),
	}.Build()
}

// ListSubtitleProfiles lists the subtitle profiles by name.
func (s *SubtitleService) ListSubtitleProfiles(ctx context.Context, _ *acquisitionv1.ListSubtitleProfilesRequest) (*acquisitionv1.ListSubtitleProfilesResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	profiles, err := s.store.SubtitleProfiles().List(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*acquisitionv1.SubtitleProfile, len(profiles))
	for i := range profiles {
		out[i] = subtitleProfileToProto(&profiles[i])
	}
	return acquisitionv1.ListSubtitleProfilesResponse_builder{Profiles: out}.Build(), nil
}

// CreateSubtitleProfile adds a subtitle profile.
func (s *SubtitleService) CreateSubtitleProfile(ctx context.Context, req *acquisitionv1.CreateSubtitleProfileRequest) (*acquisitionv1.CreateSubtitleProfileResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	p := subtitleProfileFromProto(req.GetProfile())
	p.ID = core.NewID()
	if err := s.putProfile(ctx, &p); err != nil {
		return nil, err
	}
	return acquisitionv1.CreateSubtitleProfileResponse_builder{Profile: subtitleProfileToProto(&p)}.Build(), nil
}

// UpdateSubtitleProfile replaces a subtitle profile.
func (s *SubtitleService) UpdateSubtitleProfile(ctx context.Context, req *acquisitionv1.UpdateSubtitleProfileRequest) (*acquisitionv1.UpdateSubtitleProfileResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetProfile().GetId())
	if err != nil {
		return nil, invalid(fmt.Errorf("subtitle profile ID: %w", err))
	}
	if _, err := s.store.SubtitleProfiles().Get(ctx, id); err != nil {
		return nil, connectError(ctx, err)
	}
	p := subtitleProfileFromProto(req.GetProfile())
	p.ID = id
	if err := s.putProfile(ctx, &p); err != nil {
		return nil, err
	}
	return acquisitionv1.UpdateSubtitleProfileResponse_builder{Profile: subtitleProfileToProto(&p)}.Build(), nil
}

func (s *SubtitleService) putProfile(ctx context.Context, p *core.SubtitleProfile) error {
	if err := p.Validate(); err != nil {
		return invalid(err)
	}
	if err := s.store.SubtitleProfiles().Put(ctx, p); err != nil {
		return connectError(ctx, err)
	}
	return nil
}

// DeleteSubtitleProfile deletes a subtitle profile and its assignments.
func (s *SubtitleService) DeleteSubtitleProfile(ctx context.Context, req *acquisitionv1.DeleteSubtitleProfileRequest) (*acquisitionv1.DeleteSubtitleProfileResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if err := s.store.SubtitleProfiles().Delete(ctx, core.MustParseID(req.GetId())); err != nil && !errors.Is(err, core.ErrNotFound) {
		return nil, connectError(ctx, err)
	}
	return &acquisitionv1.DeleteSubtitleProfileResponse{}, nil
}

// AssignSubtitleProfile assigns a profile to a library, a movie or a
// series; an empty profile removes the assignment.
func (s *SubtitleService) AssignSubtitleProfile(ctx context.Context, req *acquisitionv1.AssignSubtitleProfileRequest) (*acquisitionv1.AssignSubtitleProfileResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	target := core.MustParseID(req.GetTargetId())
	var err error
	if req.GetProfileId() == "" {
		if err = s.store.SubtitleProfileAssignments().Delete(ctx, target); errors.Is(err, core.ErrNotFound) {
			err = nil
		}
	} else {
		err = s.store.SubtitleProfileAssignments().Put(ctx, core.SubtitleProfileAssignment{TargetID: target, ProfileID: core.MustParseID(req.GetProfileId())})
	}
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return &acquisitionv1.AssignSubtitleProfileResponse{}, nil
}

var subtitleFolders = map[acquisitionv1.SubtitleFolder]core.SubtitleFolder{
	acquisitionv1.SubtitleFolder_SUBTITLE_FOLDER_UNSPECIFIED: core.SubtitleFolderCurrent,
	acquisitionv1.SubtitleFolder_SUBTITLE_FOLDER_RELATIVE:    core.SubtitleFolderRelative,
	acquisitionv1.SubtitleFolder_SUBTITLE_FOLDER_ABSOLUTE:    core.SubtitleFolderAbsolute,
}

func subtitleSettingsToProto(s *core.SubtitleSettings) *acquisitionv1.SubtitleSettings {
	folder := acquisitionv1.SubtitleFolder_SUBTITLE_FOLDER_UNSPECIFIED
	for k, v := range subtitleFolders {
		if v == s.Folder {
			folder = k
		}
	}
	var equals []*acquisitionv1.SubtitleLanguageEquivalence
	for _, e := range s.LanguageEquals {
		equals = append(equals, acquisitionv1.SubtitleLanguageEquivalence_builder{From: languageToProto(e.From), To: languageToProto(e.To)}.Build())
	}
	hours := func(d time.Duration) *int32 { return new(int32(d / time.Hour)) }
	days := func(d time.Duration) *int32 { return new(int32(d / (24 * time.Hour))) }
	seconds := func(d time.Duration) *int32 { return new(int32(d / time.Second)) }
	y := &s.Sync
	return acquisitionv1.SubtitleSettings_builder{
		MinimumScore: new(int32(s.MinimumScore)), MinimumScoreMovie: new(int32(s.MinimumScoreMovie)),
		UseSceneName: new(s.UseSceneName), UseEmbedded: new(s.UseEmbedded),
		IgnorePgs: new(s.IgnorePGS), IgnoreVobsub: new(s.IgnoreVobSub), IgnoreAss: new(s.IgnoreASS), Utf8: new(s.UTF8),
		Mods: s.Mods, HiExtension: new(s.HIExtension), SingleLanguage: new(s.SingleLanguage), Folder: folder.Enum(),
		CustomFolder: new(s.CustomFolder), Upgrade: new(s.Upgrade), UpgradeIntervalHours: hours(s.UpgradeInterval),
		UpgradeDays: new(int32(s.UpgradeDays)), UpgradeManual: new(s.UpgradeManual), AdaptiveSearching: new(s.AdaptiveSearching),
		AdaptiveDelayDays: days(s.AdaptiveDelay), AdaptiveDeltaDays: days(s.AdaptiveDelta),
		WantedSearchIntervalHours: hours(s.WantedSearchInterval), WantedSearchIntervalMovieHours: hours(s.WantedSearchIntervalMovie),
		DefaultAudioLanguage: new(s.DefaultAudioLanguage), DefaultEmbeddedLanguage: new(s.DefaultEmbeddedLanguage),
		SkipHashing: new(s.SkipHashing), LanguageEquals: equals,
		MovieProfileTags: new(s.MovieProfileTags), SeriesProfileTags: new(s.SeriesProfileTags), NoProfileTags: s.NoProfileTags,
		Sync: acquisitionv1.SubtitleSyncSettings_builder{
			Enabled: new(y.Enabled), UseThreshold: new(y.UseThreshold), UseMovieThreshold: new(y.UseMovieThreshold),
			Threshold: new(int32(y.Threshold)), MovieThreshold: new(int32(y.MovieThreshold)), ForceAudio: new(y.ForceAudio),
			OriginalLanguage: new(y.OriginalLanguage), ExcludedProviders: y.ExcludedProviders, ExcludedLanguages: y.ExcludedLanguages,
			MaxOffsetSeconds: seconds(y.MaxOffset), FixFramerate: new(y.FixFramerate), GoldenSection: new(y.GoldenSection),
			MinScore: new(y.MinScore), MaxQualityOffsetSeconds: seconds(y.MaxQualityOffset), MaxScaleDeviation: new(y.MaxScaleDeviation),
		}.Build(),
	}.Build()
}

func subtitleSettingsFromProto(p *acquisitionv1.SubtitleSettings) core.SubtitleSettings {
	hours := func(v int32) time.Duration { return time.Duration(v) * time.Hour }
	days := func(v int32) time.Duration { return time.Duration(v) * 24 * time.Hour }
	seconds := func(v int32) time.Duration { return time.Duration(v) * time.Second }
	var equals []core.SubtitleLanguageEquivalence
	for _, e := range p.GetLanguageEquals() {
		equals = append(equals, core.SubtitleLanguageEquivalence{From: languageFromProto(e.GetFrom()), To: languageFromProto(e.GetTo())})
	}
	y := p.GetSync()
	return core.SubtitleSettings{
		MinimumScore: int(p.GetMinimumScore()), MinimumScoreMovie: int(p.GetMinimumScoreMovie()),
		UseSceneName: p.GetUseSceneName(), UseEmbedded: p.GetUseEmbedded(),
		IgnorePGS: p.GetIgnorePgs(), IgnoreVobSub: p.GetIgnoreVobsub(), IgnoreASS: p.GetIgnoreAss(), UTF8: p.GetUtf8(),
		Mods: p.GetMods(), HIExtension: p.GetHiExtension(), SingleLanguage: p.GetSingleLanguage(),
		Folder: subtitleFolders[p.GetFolder()], CustomFolder: p.GetCustomFolder(),
		Upgrade: p.GetUpgrade(), UpgradeInterval: hours(p.GetUpgradeIntervalHours()), UpgradeDays: int(p.GetUpgradeDays()),
		UpgradeManual: p.GetUpgradeManual(), AdaptiveSearching: p.GetAdaptiveSearching(),
		AdaptiveDelay: days(p.GetAdaptiveDelayDays()), AdaptiveDelta: days(p.GetAdaptiveDeltaDays()),
		WantedSearchInterval: hours(p.GetWantedSearchIntervalHours()), WantedSearchIntervalMovie: hours(p.GetWantedSearchIntervalMovieHours()),
		DefaultAudioLanguage: p.GetDefaultAudioLanguage(), DefaultEmbeddedLanguage: p.GetDefaultEmbeddedLanguage(),
		SkipHashing: p.GetSkipHashing(), LanguageEquals: equals,
		MovieProfileTags: p.GetMovieProfileTags(), SeriesProfileTags: p.GetSeriesProfileTags(), NoProfileTags: p.GetNoProfileTags(),
		Sync: core.SubtitleSyncSettings{
			Enabled: y.GetEnabled(), UseThreshold: y.GetUseThreshold(), UseMovieThreshold: y.GetUseMovieThreshold(),
			Threshold: int(y.GetThreshold()), MovieThreshold: int(y.GetMovieThreshold()), ForceAudio: y.GetForceAudio(),
			OriginalLanguage: y.GetOriginalLanguage(), ExcludedProviders: y.GetExcludedProviders(), ExcludedLanguages: y.GetExcludedLanguages(),
			MaxOffset: seconds(y.GetMaxOffsetSeconds()), FixFramerate: y.GetFixFramerate(), GoldenSection: y.GetGoldenSection(),
			MinScore: y.GetMinScore(), MaxQualityOffset: seconds(y.GetMaxQualityOffsetSeconds()), MaxScaleDeviation: y.GetMaxScaleDeviation(),
		},
	}
}

func (s *SubtitleService) settings() core.SubtitleSettings {
	if s.Settings == nil {
		return core.DefaultSubtitleSettings()
	}
	return s.Settings.Get().Subtitles
}

// GetSubtitleSettings returns the subtitle settings.
func (s *SubtitleService) GetSubtitleSettings(ctx context.Context, _ *acquisitionv1.GetSubtitleSettingsRequest) (*acquisitionv1.GetSubtitleSettingsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	cur := s.settings()
	return acquisitionv1.GetSubtitleSettingsResponse_builder{Settings: subtitleSettingsToProto(&cur)}.Build(), nil
}

// UpdateSubtitleSettings replaces the subtitle settings. Subtitles are
// saved beside their videos, where scans find them; other folders are
// refused.
func (s *SubtitleService) UpdateSubtitleSettings(ctx context.Context, req *acquisitionv1.UpdateSubtitleSettingsRequest) (*acquisitionv1.UpdateSubtitleSettingsResponse, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	if s.Settings == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("the server keeps no settings"))
	}
	sub := subtitleSettingsFromProto(req.GetSettings())
	if err := sub.Validate(); err != nil {
		return nil, invalid(err)
	}
	for _, m := range sub.Mods {
		if err := subtitle.CheckMod(m); err != nil {
			return nil, invalid(err)
		}
	}
	all := s.Settings.Get()
	all.Subtitles = sub
	all, err = s.Settings.Update(ctx, all)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if s.Activity != nil {
		s.Activity.Record(ctx, core.Activity{Type: "settings.updated", UserID: p.User.ID, Title: p.User.Name + " changed the subtitle settings"})
	}
	return acquisitionv1.UpdateSubtitleSettingsResponse_builder{Settings: subtitleSettingsToProto(&all.Subtitles)}.Build(), nil
}

// ListSubtitleProviders lists the providers of the subtitle provider
// plugins with their state.
func (s *SubtitleService) ListSubtitleProviders(ctx context.Context, _ *acquisitionv1.ListSubtitleProvidersRequest) (*acquisitionv1.ListSubtitleProvidersResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	var out []*acquisitionv1.SubtitleProviderState
	var failed []string
	for _, p := range s.rt.Providers() {
		resp, err := p.Client.ListSubtitleProviders(ctx, &pluginv1.ListSubtitleProvidersRequest{})
		if err != nil {
			failed = append(failed, p.ID)
			continue
		}
		for _, info := range resp.GetProviders() {
			out = append(out, acquisitionv1.SubtitleProviderState_builder{
				Id: new(p.ID + "/" + info.GetName()), PluginId: new(p.ID), Name: new(info.GetName()), DisplayName: new(info.GetDisplayName()),
				Enabled: new(info.GetEnabled()), ThrottledUntil: info.GetThrottledUntil(), ThrottleReason: new(info.GetThrottleReason()),
				Languages: info.GetLanguages(), Movies: new(info.GetMovies()), Episodes: new(info.GetEpisodes()),
			}.Build())
		}
	}
	return acquisitionv1.ListSubtitleProvidersResponse_builder{Providers: out, FailedPlugins: failed}.Build(), nil
}

var subtitleItemKinds = map[acquisitionv1.SubtitleItemKind]core.ItemKind{
	acquisitionv1.SubtitleItemKind_SUBTITLE_ITEM_KIND_UNSPECIFIED: core.KindMovie,
	acquisitionv1.SubtitleItemKind_SUBTITLE_ITEM_KIND_MOVIE:       core.KindMovie,
	acquisitionv1.SubtitleItemKind_SUBTITLE_ITEM_KIND_EPISODE:     core.KindEpisode,
}

// ListWantedSubtitles lists the movies or episodes missing subtitles.
func (s *SubtitleService) ListWantedSubtitles(ctx context.Context, req *acquisitionv1.ListWantedSubtitlesRequest) (*acquisitionv1.ListWantedSubtitlesResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	wanted, total, err := s.rt.Wanted(ctx, subtitleItemKinds[req.GetKind()], int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*acquisitionv1.WantedSubtitles, len(wanted))
	for i, w := range wanted {
		b := acquisitionv1.WantedSubtitles_builder{
			ItemId: new(w.Item.ID.String()), Name: new(w.Item.Name), Missing: languagesToProto(w.Missing), Path: new(w.Path),
		}
		if w.Series != nil {
			b.SeriesId, b.SeriesName = new(w.Series.ID.String()), new(w.Series.Name)
		}
		if w.Item.ParentIndexNumber != nil {
			b.SeasonNumber = new(int32(*w.Item.ParentIndexNumber))
		}
		if w.Item.IndexNumber != nil {
			b.EpisodeNumber = new(int32(*w.Item.IndexNumber))
		}
		out[i] = b.Build()
	}
	return acquisitionv1.ListWantedSubtitlesResponse_builder{Items: out, Total: new(int32(total))}.Build(), nil
}

// SearchWantedSubtitles queues searches for the missing subtitles of
// items, or of every movie or episode when none are given.
func (s *SubtitleService) SearchWantedSubtitles(ctx context.Context, req *acquisitionv1.SearchWantedSubtitlesRequest) (*acquisitionv1.SearchWantedSubtitlesResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if len(req.GetItemIds()) == 0 {
		kind := subtitles.JobWantedMovies
		if subtitleItemKinds[req.GetKind()] == core.KindEpisode {
			kind = subtitles.JobWantedEpisodes
		}
		if err := s.rt.Queue(ctx, kind); err != nil {
			return nil, connectError(ctx, err)
		}
		return acquisitionv1.SearchWantedSubtitlesResponse_builder{Queued: new(int32(1))}.Build(), nil
	}
	ids := make([]core.ID, len(req.GetItemIds()))
	for i, id := range req.GetItemIds() {
		ids[i] = core.MustParseID(id)
	}
	n, err := s.rt.QueueSearch(ctx, ids)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return acquisitionv1.SearchWantedSubtitlesResponse_builder{Queued: new(int32(n))}.Build(), nil
}

// UpgradeSubtitles queues an upgrade of the subtitles downloaded lately.
func (s *SubtitleService) UpgradeSubtitles(ctx context.Context, _ *acquisitionv1.UpgradeSubtitlesRequest) (*acquisitionv1.UpgradeSubtitlesResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if err := s.rt.Queue(ctx, subtitles.JobUpgrade); err != nil {
		return nil, connectError(ctx, err)
	}
	return &acquisitionv1.UpgradeSubtitlesResponse{}, nil
}

// GetItemSubtitles returns a movie's or an episode's subtitles, profile
// and missing subtitles.
func (s *SubtitleService) GetItemSubtitles(ctx context.Context, req *acquisitionv1.GetItemSubtitlesRequest) (*acquisitionv1.GetItemSubtitlesResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	it, err := s.rt.Item(ctx, core.MustParseID(req.GetItemId()))
	if err != nil {
		return nil, subtitleError(ctx, err)
	}
	subs := make([]*acquisitionv1.ItemSubtitle, len(it.Streams))
	for i, st := range it.Streams {
		subs[i] = acquisitionv1.ItemSubtitle_builder{
			Language: languageToProto(st.Language), Path: new(st.Stream.ExternalPath), StreamIndex: new(int32(st.Stream.Index)),
			Codec: new(st.Stream.Codec), Counted: new(st.Counted),
		}.Build()
	}
	b := acquisitionv1.GetItemSubtitlesResponse_builder{Profile: subtitleProfileToProto(it.Profile), Subtitles: subs, Missing: languagesToProto(it.Missing)}
	if !it.Assigned.IsZero() {
		b.AssignedProfileId = new(it.Assigned.String())
	}
	return b.Build(), nil
}

// SearchItemSubtitles searches the providers for subtitles of an item and
// returns them scored.
func (s *SubtitleService) SearchItemSubtitles(ctx context.Context, req *acquisitionv1.SearchItemSubtitlesRequest) (*acquisitionv1.SearchItemSubtitlesResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	var langs []core.SubtitleLanguage
	for _, l := range req.GetLanguages() {
		langs = append(langs, languageFromProto(l))
	}
	found, failed, err := s.rt.ManualSearch(ctx, core.MustParseID(req.GetItemId()), langs)
	if err != nil {
		return nil, subtitleError(ctx, err)
	}
	out := make([]*acquisitionv1.ScoredSubtitle, len(found))
	for i, f := range found {
		pct := 0
		if f.MaxScore > 0 {
			pct = f.Score * 100 / f.MaxScore
		}
		out[i] = acquisitionv1.ScoredSubtitle_builder{
			Provider: new(f.Provider), SubtitleId: new(f.ID), Language: languageToProto(f.Language), ReleaseNames: f.ReleaseNames,
			Score: new(int32(f.Score)), MaxScore: new(int32(f.MaxScore)), Percent: new(int32(pct)),
			Matched: f.Matched, NotMatched: f.NotMatched, PageLink: new(f.PageLink), Uploader: new(f.Uploader),
			Format: new(f.Format), Blocklisted: new(f.Blocklisted),
		}.Build()
	}
	return acquisitionv1.SearchItemSubtitlesResponse_builder{Subtitles: out, FailedProviders: failed}.Build(), nil
}

// DownloadItemSubtitle downloads a subtitle a search found.
func (s *SubtitleService) DownloadItemSubtitle(ctx context.Context, req *acquisitionv1.DownloadItemSubtitleRequest) (*acquisitionv1.DownloadItemSubtitleResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	e, err := s.rt.ManualDownload(ctx, core.MustParseID(req.GetItemId()), req.GetProvider(), req.GetSubtitleId(), languageFromProto(req.GetLanguage()))
	if err != nil {
		return nil, subtitleError(ctx, err)
	}
	return acquisitionv1.DownloadItemSubtitleResponse_builder{Path: new(e.Path)}.Build(), nil
}

// UploadSubtitle saves an uploaded subtitle beside an item's video.
func (s *SubtitleService) UploadSubtitle(ctx context.Context, req *acquisitionv1.UploadSubtitleRequest) (*acquisitionv1.UploadSubtitleResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	p, err := s.rt.Upload(ctx, core.MustParseID(req.GetItemId()), req.GetData(), req.GetFormat(), languageFromProto(req.GetLanguage()))
	if err != nil {
		return nil, subtitleError(ctx, err)
	}
	return acquisitionv1.UploadSubtitleResponse_builder{Path: new(p)}.Build(), nil
}

// DeleteSubtitle deletes a subtitle file of an item.
func (s *SubtitleService) DeleteSubtitle(ctx context.Context, req *acquisitionv1.DeleteSubtitleRequest) (*acquisitionv1.DeleteSubtitleResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if err := s.rt.Delete(ctx, core.MustParseID(req.GetItemId()), req.GetPath()); err != nil {
		return nil, subtitleError(ctx, err)
	}
	return &acquisitionv1.DeleteSubtitleResponse{}, nil
}

// ModifySubtitle applies subtitle mods to a subtitle file of an item.
func (s *SubtitleService) ModifySubtitle(ctx context.Context, req *acquisitionv1.ModifySubtitleRequest) (*acquisitionv1.ModifySubtitleResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if err := s.rt.Modify(ctx, core.MustParseID(req.GetItemId()), req.GetPath(), req.GetMods()); err != nil {
		return nil, subtitleError(ctx, err)
	}
	return &acquisitionv1.ModifySubtitleResponse{}, nil
}

// SyncSubtitle synchronizes a subtitle file of an item against its
// video's audio.
func (s *SubtitleService) SyncSubtitle(ctx context.Context, req *acquisitionv1.SyncSubtitleRequest) (*acquisitionv1.SyncSubtitleResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if req.GetReferencePath() != "" {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("subtitles are synchronized against the audio only"))
	}
	res, err := s.rt.Sync(ctx, core.MustParseID(req.GetItemId()), req.GetPath())
	if err != nil {
		return nil, subtitleError(ctx, err)
	}
	return acquisitionv1.SyncSubtitleResponse_builder{OffsetMs: new(res.Offset.Milliseconds()), Ratio: new(res.Scale)}.Build(), nil
}

// BlocklistSubtitle blocklists the subtitle a history event downloaded,
// deletes it and searches again.
func (s *SubtitleService) BlocklistSubtitle(ctx context.Context, req *acquisitionv1.BlocklistSubtitleRequest) (*acquisitionv1.BlocklistSubtitleResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if err := s.rt.BlocklistEvent(ctx, core.MustParseID(req.GetHistoryEventId())); err != nil {
		return nil, subtitleError(ctx, err)
	}
	return &acquisitionv1.BlocklistSubtitleResponse{}, nil
}

var subtitleActions = map[acquisitionv1.SubtitleAction]core.SubtitleAction{
	acquisitionv1.SubtitleAction_SUBTITLE_ACTION_DELETED:    core.SubtitleDeleted,
	acquisitionv1.SubtitleAction_SUBTITLE_ACTION_DOWNLOADED: core.SubtitleDownloaded,
	acquisitionv1.SubtitleAction_SUBTITLE_ACTION_MANUAL:     core.SubtitleManual,
	acquisitionv1.SubtitleAction_SUBTITLE_ACTION_UPGRADED:   core.SubtitleUpgraded,
	acquisitionv1.SubtitleAction_SUBTITLE_ACTION_UPLOADED:   core.SubtitleUploaded,
	acquisitionv1.SubtitleAction_SUBTITLE_ACTION_SYNCED:     core.SubtitleSynced,
	acquisitionv1.SubtitleAction_SUBTITLE_ACTION_EXTRACTED:  core.SubtitleExtracted,
}

func subtitleEventToProto(e *core.SubtitleHistoryEvent) *acquisitionv1.SubtitleHistoryEvent {
	action := acquisitionv1.SubtitleAction_SUBTITLE_ACTION_UNSPECIFIED
	for k, v := range subtitleActions {
		if v == e.Action {
			action = k
		}
	}
	b := acquisitionv1.SubtitleHistoryEvent_builder{
		Id: new(e.ID.String()), Action: action.Enum(), ItemId: new(e.ItemID.String()), Time: timestamppb.New(e.Date),
		Language: languageToProto(e.Language), Provider: new(e.Provider), SubtitleId: new(e.SubtitleID),
		Score: new(int32(e.Score)), MaxScore: new(int32(e.MaxScore)), Path: new(e.Path), VideoPath: new(e.VideoPath),
		Description: new(e.Description), Matched: e.Matched, NotMatched: e.NotMatched,
	}
	if !e.ParentID.IsZero() {
		b.SeriesId = new(e.ParentID.String())
	}
	if !e.UpgradedFrom.IsZero() {
		b.UpgradedFrom = new(e.UpgradedFrom.String())
	}
	return b.Build()
}

// ListSubtitleHistory lists subtitle history, newest first.
func (s *SubtitleService) ListSubtitleHistory(ctx context.Context, req *acquisitionv1.ListSubtitleHistoryRequest) (*acquisitionv1.ListSubtitleHistoryResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	q := core.SubtitleHistoryQuery{Limit: int(req.GetLimit()), Offset: int(req.GetOffset())}
	for _, id := range req.GetItemIds() {
		q.ItemIDs = append(q.ItemIDs, core.MustParseID(id))
	}
	for _, a := range req.GetActions() {
		q.Actions = append(q.Actions, subtitleActions[a])
	}
	page, err := s.store.SubtitleHistory().List(ctx, q)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*acquisitionv1.SubtitleHistoryEvent, len(page.Items))
	for i := range page.Items {
		out[i] = subtitleEventToProto(&page.Items[i])
	}
	return acquisitionv1.ListSubtitleHistoryResponse_builder{Events: out, Total: new(int32(page.Total))}.Build(), nil
}

// ListSubtitleBlocklist lists blocklisted subtitles, newest first.
func (s *SubtitleService) ListSubtitleBlocklist(ctx context.Context, req *acquisitionv1.ListSubtitleBlocklistRequest) (*acquisitionv1.ListSubtitleBlocklistResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	var item core.ID
	if req.GetItemId() != "" {
		item = core.MustParseID(req.GetItemId())
	}
	page, err := s.store.SubtitleBlocklist().List(ctx, item, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*acquisitionv1.SubtitleBlocklistEntry, len(page.Items))
	for i, b := range page.Items {
		eb := acquisitionv1.SubtitleBlocklistEntry_builder{
			Id: new(b.ID.String()), ItemId: new(b.ItemID.String()), Provider: new(b.Provider), SubtitleId: new(b.SubtitleID),
			Language: languageToProto(b.Language), Time: timestamppb.New(b.Date), Message: new(b.Message),
		}
		if !b.ParentID.IsZero() {
			eb.SeriesId = new(b.ParentID.String())
		}
		out[i] = eb.Build()
	}
	return acquisitionv1.ListSubtitleBlocklistResponse_builder{Entries: out, Total: new(int32(page.Total))}.Build(), nil
}

// DeleteSubtitleBlocklist removes blocklisted subtitles, or all of them.
func (s *SubtitleService) DeleteSubtitleBlocklist(ctx context.Context, req *acquisitionv1.DeleteSubtitleBlocklistRequest) (*acquisitionv1.DeleteSubtitleBlocklistResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	var err error
	if req.GetAll() {
		err = s.store.SubtitleBlocklist().Clear(ctx)
	} else {
		ids := make([]core.ID, len(req.GetIds()))
		for i, id := range req.GetIds() {
			ids[i] = core.MustParseID(id)
		}
		err = s.store.SubtitleBlocklist().Delete(ctx, ids...)
	}
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return &acquisitionv1.DeleteSubtitleBlocklistResponse{}, nil
}
