package app

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/proto"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// optionsForm is the Options page's draft: the settings loaded, those as
// the draft started (with naming settings), and as edited, the lists as
// the text typed.
type optionsForm struct {
	loaded, from, draft     *acquisitionv1.AcquisitionSettings
	extensions, whitelisted string
}

// namingDefaults are the episode and folder templates' defaults, which
// empty ones use.
var namingDefaults = map[string]string{
	"standard": "{Series Title} - S{season:00}E{episode:00} - {Episode Title} {Quality Full}",
	"daily":    "{Series Title} - {Air-Date} - {Episode Title} {Quality Full}",
	"anime":    "{Series Title} - S{season:00}E{episode:00} - {Episode Title} {Quality Full}",
	"series":   "{Series Title}",
	"season":   "Season {season}",
	"specials": "Specials",
}

// enumChoice is a value of an enumeration and its label, a key of the
// strings (with a dot) or the label itself.
type enumChoice[E ~int32] struct {
	value E
	label string
}

var (
	colonChoices = []enumChoice[acquisitionv1.ColonReplacement]{
		{acquisitionv1.ColonReplacement_COLON_REPLACEMENT_SMART, "options.colonSmart"},
		{acquisitionv1.ColonReplacement_COLON_REPLACEMENT_DELETE, "options.colonDelete"},
		{acquisitionv1.ColonReplacement_COLON_REPLACEMENT_DASH, "options.colonDash"},
		{acquisitionv1.ColonReplacement_COLON_REPLACEMENT_SPACE_DASH, "options.colonSpaceDash"},
		{acquisitionv1.ColonReplacement_COLON_REPLACEMENT_SPACE_DASH_SPACE, "options.colonSpaceDashSpace"},
	}
	// The multi-episode styles, shown for episodes 6 to 8 of season 15.
	multiEpisodeChoices = []enumChoice[acquisitionv1.MultiEpisodeStyle]{
		{acquisitionv1.MultiEpisodeStyle_MULTI_EPISODE_STYLE_PREFIXED_RANGE, "S15E06-E08"},
		{acquisitionv1.MultiEpisodeStyle_MULTI_EPISODE_STYLE_RANGE, "S15E06-08"},
		{acquisitionv1.MultiEpisodeStyle_MULTI_EPISODE_STYLE_EXTEND, "S15E06-07-08"},
		{acquisitionv1.MultiEpisodeStyle_MULTI_EPISODE_STYLE_SCENE, "S15E06-E07-E08"},
		{acquisitionv1.MultiEpisodeStyle_MULTI_EPISODE_STYLE_REPEAT, "S15E06E07E08"},
		{acquisitionv1.MultiEpisodeStyle_MULTI_EPISODE_STYLE_DUPLICATE, "S15E06 - S15E07 - S15E08"},
	}
	titleChoices = []enumChoice[acquisitionv1.EpisodeTitleRequired]{
		{acquisitionv1.EpisodeTitleRequired_EPISODE_TITLE_REQUIRED_ALWAYS, "options.titleAlways"},
		{acquisitionv1.EpisodeTitleRequired_EPISODE_TITLE_REQUIRED_BULK_SEASON_RELEASES, "options.titleBulk"},
		{acquisitionv1.EpisodeTitleRequired_EPISODE_TITLE_REQUIRED_NEVER, "options.titleNever"},
	}
	properChoices = []enumChoice[acquisitionv1.ProperDownload]{
		{acquisitionv1.ProperDownload_PROPER_DOWNLOAD_PREFER_AND_UPGRADE, "options.propersUpgrade"},
		{acquisitionv1.ProperDownload_PROPER_DOWNLOAD_DO_NOT_UPGRADE, "options.propersNoUpgrade"},
		{acquisitionv1.ProperDownload_PROPER_DOWNLOAD_DO_NOT_PREFER, "options.propersNoPrefer"},
	}
)

// chooseEnum builds a choice among an enumeration's values, the unset
// value showing as fallback, and reports a new one.
func chooseEnum[E ~int32](c *ui.Context, label string, value *E, fallback E, list []enumChoice[E]) bool {
	labels := make([]string, len(list))
	i := 0
	for j, x := range list {
		labels[j] = x.label
		if strings.Contains(x.label, ".") {
			labels[j] = style.T(c, x.label)
		}
		if x.value == *value || (*value == 0 && x.value == fallback) {
			i = j
		}
	}
	if choose(c, label, &i, labels) {
		*value = list[i].value
		return true
	}
	return false
}

// acquisitionOptionsPage builds the Options: how the server names what it
// imports, how it imports finished downloads, how often it checks
// download clients, which releases it takes, and the recycle bin.
func (a *App) acquisitionOptionsPage(c *ui.Context) {
	st := &a.settingsState.admin.acquisition
	f := &st.options
	a.backTo(c, style.T(c, "settings.acquisition"), acquisitionPath("", ""))
	sectionHeader(c, style.T(c, "options.options"), style.T(c, "options.optionsMessage"))
	t := adminTask(a, &st.settings, "acquisition-settings", func(ctx context.Context, s string) (*acquisitionv1.AcquisitionSettings, error) {
		res, err := a.profileService(s).GetAcquisitionSettings(ctx, &acquisitionv1.GetAcquisitionSettingsRequest{})
		return res.GetSettings(), err
	})
	if taskStatus(c, t, func() { st.settings.key = "" }, nil) {
		return
	}
	// The settings saved: the draft with its lists, the text fields
	// trimmed, set even when empty.
	saved := func() *acquisitionv1.AcquisitionSettings {
		s := proto.CloneOf(f.draft)
		s.SetExtraFileExtensions(splitList(f.extensions, ","))
		s.SetWhitelistedHardcodedSubs(splitList(f.whitelisted, ","))
		s.SetRecycleBin(strings.TrimSpace(s.GetRecycleBin()))
		s.SetDownloadClientWorkingFolders(strings.TrimSpace(s.GetDownloadClientWorkingFolders()))
		return s
	}
	if t.value != f.loaded && (f.loaded == nil || proto.Equal(saved(), f.from)) {
		f.loaded, f.draft = t.value, proto.CloneOf(t.value)
		if !f.draft.HasNaming() {
			f.draft.SetNaming(&acquisitionv1.NamingSettings{})
		}
		f.extensions = strings.Join(t.value.GetExtraFileExtensions(), ", ")
		f.whitelisted = strings.Join(t.value.GetWhitelistedHardcodedSubs(), ", ")
		f.from = saved()
	}
	s, n := f.draft, f.draft.GetNaming()
	changed := !proto.Equal(saved(), f.from)
	if changed {
		st.notice = ""
	}
	toggle := func(g *settingsGroup, label, detail string, get func() bool, set func(bool)) {
		on := get()
		if g.switchRow(label, detail, &on) {
			set(on)
		}
	}
	number := func(g *settingsGroup, label, detail string, value int32, lo, hi float64, set func(int32)) {
		g.row(label, detail, nil, func() {
			numberField(c, label, float64(value), lo, hi, 1, func(v float64) { set(int32(v)) })
		})
	}
	text := func(g *settingsGroup, label, detail, placeholder string, value *string, width float32) {
		g.row(label, detail, nil, func() {
			ui.TextInput(c, value).Width(width).Label(label).Placeholder(placeholder)
		})
	}
	template := func(g *settingsGroup, label, placeholder string, get func() string, set func(string)) {
		v := get()
		text(g, label, "", placeholder, &v, 384)
		if v != get() {
			set(v)
		}
	}
	enum := func(g *settingsGroup, label, detail string, control func()) {
		g.row(label, detail, nil, control)
	}
	group(c, "naming", style.T(c, "options.naming"), "", func(g *settingsGroup) {
		toggle(g, style.T(c, "options.replaceIllegal"), style.T(c, "options.replaceIllegalDetail"), n.GetReplaceIllegalCharacters, n.SetReplaceIllegalCharacters)
		enum(g, style.T(c, "options.colon"), "", func() {
			v := n.GetColonReplacement()
			if chooseEnum(c, style.T(c, "options.colon"), &v, acquisitionv1.ColonReplacement_COLON_REPLACEMENT_SMART, colonChoices) {
				n.SetColonReplacement(v)
			}
		})
	})
	group(c, "movies", style.T(c, "options.movieNaming"), "", func(g *settingsGroup) {
		toggle(g, style.T(c, "options.renameMovies"), style.T(c, "options.renameMoviesDetail"), n.GetRenameMovies, n.SetRenameMovies)
		template(g, style.T(c, "options.movieFolder"), "", n.GetMovieFolderFormat, n.SetMovieFolderFormat)
		template(g, style.T(c, "options.movieFile"), "", n.GetStandardMovieFormat, n.SetStandardMovieFormat)
	})
	group(c, "episodes", style.T(c, "options.episodeNaming"), style.T(c, "options.namingFooter"), func(g *settingsGroup) {
		toggle(g, style.T(c, "options.renameEpisodes"), style.T(c, "options.renameMoviesDetail"), n.GetRenameEpisodes, n.SetRenameEpisodes)
		template(g, style.T(c, "options.standardEpisode"), namingDefaults["standard"], n.GetStandardEpisodeFormat, n.SetStandardEpisodeFormat)
		template(g, style.T(c, "options.dailyEpisode"), namingDefaults["daily"], n.GetDailyEpisodeFormat, n.SetDailyEpisodeFormat)
		template(g, style.T(c, "options.animeEpisode"), namingDefaults["anime"], n.GetAnimeEpisodeFormat, n.SetAnimeEpisodeFormat)
		template(g, style.T(c, "options.seriesFolder"), namingDefaults["series"], n.GetSeriesFolderFormat, n.SetSeriesFolderFormat)
		template(g, style.T(c, "options.seasonFolder"), namingDefaults["season"], n.GetSeasonFolderFormat, n.SetSeasonFolderFormat)
		template(g, style.T(c, "options.specialsFolder"), namingDefaults["specials"], n.GetSpecialsFolderFormat, n.SetSpecialsFolderFormat)
		enum(g, style.T(c, "options.multiEpisode"), "", func() {
			v := n.GetMultiEpisodeStyle()
			if chooseEnum(c, style.T(c, "options.multiEpisode"), &v, acquisitionv1.MultiEpisodeStyle_MULTI_EPISODE_STYLE_PREFIXED_RANGE, multiEpisodeChoices) {
				n.SetMultiEpisodeStyle(v)
			}
		})
	})
	group(c, "importing", style.T(c, "options.importing"), "", func(g *settingsGroup) {
		toggle(g, style.T(c, "options.completedHandling"), style.T(c, "options.completedHandlingDetail"), s.GetCompletedDownloadHandling, s.SetCompletedDownloadHandling)
		toggle(g, style.T(c, "options.hardlinks"), style.T(c, "options.hardlinksDetail"), s.GetCopyUsingHardlinks, s.SetCopyUsingHardlinks)
		toggle(g, style.T(c, "options.extraFiles"), "", s.GetImportExtraFiles, s.SetImportExtraFiles)
		if s.GetImportExtraFiles() {
			text(g, style.T(c, "options.extensions"), style.T(c, "options.extensionsDetail"), "srt", &f.extensions, 288)
		}
		toggle(g, style.T(c, "options.deleteEmpty"), "", s.GetDeleteEmptyFolders, s.SetDeleteEmptyFolders)
		toggle(g, style.T(c, "options.skipFreeSpace"), "", s.GetSkipFreeSpaceCheck, s.SetSkipFreeSpaceCheck)
		if !s.GetSkipFreeSpaceCheck() {
			number(g, style.T(c, "options.minimumFreeSpace"), style.T(c, "options.minimumFreeSpaceDetail"), s.GetMinimumFreeSpace(), 0, 1e9, s.SetMinimumFreeSpace)
		}
		enum(g, style.T(c, "options.episodeTitle"), style.T(c, "options.episodeTitleDetail"), func() {
			v := s.GetEpisodeTitleRequired()
			if chooseEnum(c, style.T(c, "options.episodeTitle"), &v, acquisitionv1.EpisodeTitleRequired_EPISODE_TITLE_REQUIRED_ALWAYS, titleChoices) {
				s.SetEpisodeTitleRequired(v)
			}
		})
		toggle(g, style.T(c, "options.unmonitorMissing"), style.T(c, "options.unmonitorMissingDetail"),
			s.GetAutoUnmonitorPreviouslyDownloadedEpisodes, s.SetAutoUnmonitorPreviouslyDownloadedEpisodes)
		v := s.GetDownloadClientWorkingFolders()
		text(g, style.T(c, "options.workingFolders"), style.T(c, "options.workingFoldersDetail"), "_UNPACK_|_FAILED_", &v, 288)
		s.SetDownloadClientWorkingFolders(v)
	})
	group(c, "clients", style.T(c, "options.downloadClients"), "", func(g *settingsGroup) {
		number(g, style.T(c, "options.checkInterval"), style.T(c, "options.checkIntervalDetail"), s.GetCheckFinishedInterval(), 1, 120, s.SetCheckFinishedInterval)
		toggle(g, style.T(c, "options.redownload"), "", s.GetAutoRedownloadFailed, s.SetAutoRedownloadFailed)
		if s.GetAutoRedownloadFailed() {
			toggle(g, style.T(c, "options.redownloadInteractive"), "", s.GetAutoRedownloadFailedFromInteractiveSearch, s.SetAutoRedownloadFailedFromInteractiveSearch)
		}
	})
	group(c, "releases", style.T(c, "options.releasesGroup"), "", func(g *settingsGroup) {
		enum(g, style.T(c, "options.propers"), "", func() {
			v := s.GetPropers()
			if chooseEnum(c, style.T(c, "options.propers"), &v, acquisitionv1.ProperDownload_PROPER_DOWNLOAD_PREFER_AND_UPGRADE, properChoices) {
				s.SetPropers(v)
			}
		})
		toggle(g, style.T(c, "options.indexerFlags"), "", s.GetPreferIndexerFlags, s.SetPreferIndexerFlags)
		number(g, style.T(c, "options.rssInterval"), style.T(c, "options.rssIntervalDetail"), s.GetRssSyncInterval(), 0, 120, s.SetRssSyncInterval)
		number(g, style.T(c, "options.maximumSize"), style.T(c, "options.maximumSizeDetail"), s.GetMaximumSize(), 0, 1e9, s.SetMaximumSize)
		number(g, style.T(c, "options.minimumAge"), style.T(c, "options.minimumAgeDetail"), s.GetMinimumAge(), 0, 1e9, s.SetMinimumAge)
		number(g, style.T(c, "options.retention"), style.T(c, "options.retentionDetail"), s.GetRetention(), 0, 1e9, s.SetRetention)
		number(g, style.T(c, "options.availabilityDelay"), style.T(c, "options.availabilityDelayDetail"), s.GetAvailabilityDelay(), -365, 365, s.SetAvailabilityDelay)
		toggle(g, style.T(c, "options.hardcodedSubs"), "", s.GetAllowHardcodedSubs, s.SetAllowHardcodedSubs)
		if !s.GetAllowHardcodedSubs() {
			text(g, style.T(c, "options.whitelistedSubs"), style.T(c, "options.whitelistedSubsDetail"), "kor, chi", &f.whitelisted, 288)
		}
	})
	group(c, "recycle", style.T(c, "options.recycleBin"), "", func(g *settingsGroup) {
		v := s.GetRecycleBin()
		text(g, style.T(c, "options.recycleBin"), style.T(c, "options.recycleBinDetail"), "/data/recycle", &v, 288)
		s.SetRecycleBin(v)
		if strings.TrimSpace(v) != "" {
			number(g, style.T(c, "options.cleanupDays"), style.T(c, "options.cleanupDaysDetail"), s.GetRecycleBinCleanupDays(), 0, 1e9, s.SetRecycleBinCleanupDays)
		}
	})
	a.acquisitionSave(c, style.T(c, "action.save"), changed, changed, func() {
		settings := saved()
		a.acquisitionAction("save", func(ctx context.Context, server string) error {
			_, err := a.profileService(server).UpdateAcquisitionSettings(ctx, acquisitionv1.UpdateAcquisitionSettingsRequest_builder{Settings: settings}.Build())
			return err
		}, func() {
			f.loaded, st.notice = nil, "acquisition.saved"
		})
	})
}
