package rpc

import (
	"context"

	"github.com/mavioai/mavio/libs/core"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
)

var properDownloads = map[core.ProperDownload]acquisitionv1.ProperDownload{
	core.PropersPreferAndUpgrade: acquisitionv1.ProperDownload_PROPER_DOWNLOAD_PREFER_AND_UPGRADE,
	core.PropersDoNotUpgrade:     acquisitionv1.ProperDownload_PROPER_DOWNLOAD_DO_NOT_UPGRADE,
	core.PropersDoNotPrefer:      acquisitionv1.ProperDownload_PROPER_DOWNLOAD_DO_NOT_PREFER,
}

var colonReplacements = map[core.ColonReplacement]acquisitionv1.ColonReplacement{
	core.ColonDelete:         acquisitionv1.ColonReplacement_COLON_REPLACEMENT_DELETE,
	core.ColonDash:           acquisitionv1.ColonReplacement_COLON_REPLACEMENT_DASH,
	core.ColonSpaceDash:      acquisitionv1.ColonReplacement_COLON_REPLACEMENT_SPACE_DASH,
	core.ColonSpaceDashSpace: acquisitionv1.ColonReplacement_COLON_REPLACEMENT_SPACE_DASH_SPACE,
	core.ColonSmart:          acquisitionv1.ColonReplacement_COLON_REPLACEMENT_SMART,
}

var multiEpisodeStyles = map[core.MultiEpisodeStyle]acquisitionv1.MultiEpisodeStyle{
	core.MultiEpisodeExtend:        acquisitionv1.MultiEpisodeStyle_MULTI_EPISODE_STYLE_EXTEND,
	core.MultiEpisodeDuplicate:     acquisitionv1.MultiEpisodeStyle_MULTI_EPISODE_STYLE_DUPLICATE,
	core.MultiEpisodeRepeat:        acquisitionv1.MultiEpisodeStyle_MULTI_EPISODE_STYLE_REPEAT,
	core.MultiEpisodeScene:         acquisitionv1.MultiEpisodeStyle_MULTI_EPISODE_STYLE_SCENE,
	core.MultiEpisodeRange:         acquisitionv1.MultiEpisodeStyle_MULTI_EPISODE_STYLE_RANGE,
	core.MultiEpisodePrefixedRange: acquisitionv1.MultiEpisodeStyle_MULTI_EPISODE_STYLE_PREFIXED_RANGE,
}

var episodeTitleRequirements = map[core.EpisodeTitleRequired]acquisitionv1.EpisodeTitleRequired{
	core.EpisodeTitleAlways:             acquisitionv1.EpisodeTitleRequired_EPISODE_TITLE_REQUIRED_ALWAYS,
	core.EpisodeTitleBulkSeasonReleases: acquisitionv1.EpisodeTitleRequired_EPISODE_TITLE_REQUIRED_BULK_SEASON_RELEASES,
	core.EpisodeTitleNever:              acquisitionv1.EpisodeTitleRequired_EPISODE_TITLE_REQUIRED_NEVER,
}

func acquisitionSettingsToProto(a *core.AcquisitionSettings) *acquisitionv1.AcquisitionSettings {
	n := &a.Naming
	return acquisitionv1.AcquisitionSettings_builder{
		Naming: acquisitionv1.NamingSettings_builder{
			RenameMovies: &n.RenameMovies, RenameEpisodes: &n.RenameEpisodes, ReplaceIllegalCharacters: &n.ReplaceIllegalCharacters,
			ColonReplacement:  new(colonReplacements[n.ColonReplacement]),
			MovieFolderFormat: &n.MovieFolderFormat, StandardMovieFormat: &n.StandardMovieFormat,
			StandardEpisodeFormat: &n.StandardEpisodeFormat, DailyEpisodeFormat: &n.DailyEpisodeFormat, AnimeEpisodeFormat: &n.AnimeEpisodeFormat,
			SeriesFolderFormat: &n.SeriesFolderFormat, SeasonFolderFormat: &n.SeasonFolderFormat, SpecialsFolderFormat: &n.SpecialsFolderFormat,
			MultiEpisodeStyle: new(multiEpisodeStyles[n.MultiEpisodeStyle]),
		}.Build(),
		CompletedDownloadHandling: &a.CompletedDownloadHandling, CopyUsingHardlinks: &a.CopyUsingHardlinks,
		ImportExtraFiles: &a.ImportExtraFiles, ExtraFileExtensions: a.ExtraFileExtensions, DeleteEmptyFolders: &a.DeleteEmptyFolders,
		SkipFreeSpaceCheck: &a.SkipFreeSpaceCheck, MinimumFreeSpace: new(int32(a.MinimumFreeSpace)),
		EpisodeTitleRequired:                      new(episodeTitleRequirements[a.EpisodeTitleRequired]),
		AutoUnmonitorPreviouslyDownloadedEpisodes: &a.AutoUnmonitorPreviouslyDownloadedEpisodes,
		DownloadClientWorkingFolders:              &a.DownloadClientWorkingFolders,
		CheckFinishedInterval:                     new(int32(a.CheckFinishedInterval)), AutoRedownloadFailed: &a.AutoRedownloadFailed,
		AutoRedownloadFailedFromInteractiveSearch: &a.AutoRedownloadFailedFromInteractiveSearch,
		Propers: new(properDownloads[a.Propers]), PreferIndexerFlags: &a.PreferIndexerFlags,
		MaximumSize: new(int32(a.MaximumSize)), MinimumAge: new(int32(a.MinimumAge)), Retention: new(int32(a.Retention)),
		RssSyncInterval: new(int32(a.RSSSyncInterval)), AvailabilityDelay: new(int32(a.AvailabilityDelay)),
		AllowHardcodedSubs: &a.AllowHardcodedSubs, WhitelistedHardcodedSubs: a.WhitelistedHardcodedSubs,
		RecycleBin: &a.RecycleBin, RecycleBinCleanupDays: new(int32(a.RecycleBinCleanupDays)),
	}.Build()
}

func acquisitionSettingsFromProto(p *acquisitionv1.AcquisitionSettings) core.AcquisitionSettings {
	n := p.GetNaming()
	return core.AcquisitionSettings{
		Naming: core.NamingSettings{
			RenameMovies: n.GetRenameMovies(), RenameEpisodes: n.GetRenameEpisodes(), ReplaceIllegalCharacters: n.GetReplaceIllegalCharacters(),
			ColonReplacement:  keyOf(colonReplacements, n.GetColonReplacement()),
			MovieFolderFormat: n.GetMovieFolderFormat(), StandardMovieFormat: n.GetStandardMovieFormat(),
			StandardEpisodeFormat: n.GetStandardEpisodeFormat(), DailyEpisodeFormat: n.GetDailyEpisodeFormat(), AnimeEpisodeFormat: n.GetAnimeEpisodeFormat(),
			SeriesFolderFormat: n.GetSeriesFolderFormat(), SeasonFolderFormat: n.GetSeasonFolderFormat(), SpecialsFolderFormat: n.GetSpecialsFolderFormat(),
			MultiEpisodeStyle: keyOf(multiEpisodeStyles, n.GetMultiEpisodeStyle()),
		},
		CompletedDownloadHandling: p.GetCompletedDownloadHandling(), CopyUsingHardlinks: p.GetCopyUsingHardlinks(),
		ImportExtraFiles: p.GetImportExtraFiles(), ExtraFileExtensions: p.GetExtraFileExtensions(), DeleteEmptyFolders: p.GetDeleteEmptyFolders(),
		SkipFreeSpaceCheck: p.GetSkipFreeSpaceCheck(), MinimumFreeSpace: int(p.GetMinimumFreeSpace()),
		EpisodeTitleRequired:                      keyOf(episodeTitleRequirements, p.GetEpisodeTitleRequired()),
		AutoUnmonitorPreviouslyDownloadedEpisodes: p.GetAutoUnmonitorPreviouslyDownloadedEpisodes(),
		DownloadClientWorkingFolders:              p.GetDownloadClientWorkingFolders(),
		CheckFinishedInterval:                     int(p.GetCheckFinishedInterval()), AutoRedownloadFailed: p.GetAutoRedownloadFailed(),
		AutoRedownloadFailedFromInteractiveSearch: p.GetAutoRedownloadFailedFromInteractiveSearch(),
		Propers: keyOf(properDownloads, p.GetPropers()), PreferIndexerFlags: p.GetPreferIndexerFlags(),
		MaximumSize: int(p.GetMaximumSize()), MinimumAge: int(p.GetMinimumAge()), Retention: int(p.GetRetention()),
		RSSSyncInterval: int(p.GetRssSyncInterval()), AvailabilityDelay: int(p.GetAvailabilityDelay()),
		AllowHardcodedSubs: p.GetAllowHardcodedSubs(), WhitelistedHardcodedSubs: p.GetWhitelistedHardcodedSubs(),
		RecycleBin: p.GetRecycleBin(), RecycleBinCleanupDays: int(p.GetRecycleBinCleanupDays()),
	}
}

// GetAcquisitionSettings returns the acquisition settings.
func (s *ProfileService) GetAcquisitionSettings(ctx context.Context, _ *acquisitionv1.GetAcquisitionSettingsRequest) (*acquisitionv1.GetAcquisitionSettingsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	set := currentSettings(s.Settings)
	return acquisitionv1.GetAcquisitionSettingsResponse_builder{Settings: acquisitionSettingsToProto(&set.Acquisition)}.Build(), nil
}

// UpdateAcquisitionSettings applies and stores the acquisition settings.
func (s *ProfileService) UpdateAcquisitionSettings(ctx context.Context, req *acquisitionv1.UpdateAcquisitionSettingsRequest) (*acquisitionv1.UpdateAcquisitionSettingsResponse, error) {
	set, err := changeSettings(ctx, s.Settings, s.Activity, "the acquisition settings", func(cur *core.ServerSettings) error {
		cur.Acquisition = acquisitionSettingsFromProto(req.GetSettings())
		return nil
	})
	if err != nil {
		return nil, err
	}
	return acquisitionv1.UpdateAcquisitionSettingsResponse_builder{Settings: acquisitionSettingsToProto(&set.Acquisition)}.Build(), nil
}
