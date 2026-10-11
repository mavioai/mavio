package acquisition

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/acquire/organize"
	"github.com/mavioai/mavio/libs/acquire/series"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// The warnings of Sonarr's episode import specifications.
const (
	msgUnpacking          = "File is still being unpacked"
	msgEpisodeUnexpected  = "Episode %s was unexpected considering the %s folder name"
	msgEpisodesUnexpected = "Episodes %s were unexpected considering the %s folder name"
	msgTitleMissing       = "Episode does not have a title and recently aired"
	msgTitleTBA           = "Episode has a TBA title and recently aired"
	msgUnverifiedScene    = "This show has individual episode mappings on TheXEM but the mapping for this episode has not been confirmed yet by their administrators. TheXEM needs manual input."
	msgNoAbsoluteNumber   = "Episode does not have an absolute episode number and recently aired"
)

// downloadWorkingFolders are the prefixes of the folders download clients
// unpack into or leave failed downloads in, Sonarr's default
// DownloadClientWorkingFolders.
const downloadWorkingFolders = "_UNPACK_|_FAILED_"

// unpacking reports whether a file is still being unpacked, as Sonarr's
// NotUnpackingSpecification: a folder it is in starts with a working
// folder's name; on Windows, only while it was written within the last
// five minutes.
func unpacking(path string, now time.Time) bool {
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		name := filepath.Base(dir)
		for _, working := range strings.Split(downloadWorkingFolders, "|") {
			if !strings.HasPrefix(name, working) {
				continue
			}
			if runtime.GOOS != "windows" {
				return true
			}
			if fi, err := os.Stat(path); err == nil && fi.ModTime().After(now.Add(-5*time.Minute)) {
				return true
			}
		}
		if parent := filepath.Dir(dir); parent == dir {
			return false
		}
	}
}

// sceneNameOf returns the release name of a file, as Sonarr's
// SceneNameCalculator: the download's title for a lone file of a
// download that is not a full season, else the file's name or the
// download folder's when they are scene names; empty when there is none.
func sceneNameOf(path string, single bool, downloadInfo, folderInfo *release.ParsedEpisodeInfo) string {
	if single && downloadInfo != nil && !downloadInfo.FullSeason {
		return release.RemoveFileExtension(downloadInfo.ReleaseTitle, release.Series)
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if release.IsSceneTitle(name, release.Series) {
		return name
	}
	if single && folderInfo != nil && !folderInfo.FullSeason && release.IsSceneTitle(folderInfo.ReleaseTitle, release.Series) {
		return folderInfo.ReleaseTitle
	}
	return ""
}

// unexpectedEpisodes returns why a file's episodes are not those of the
// download folder it is in, as Sonarr's MatchesFolderSpecification; empty
// when they are, or when either name holds no episodes.
func unexpectedEpisodes(ctx context.Context, ps *series.Parsing, s *series.Series, fileInfo, folderInfo *release.ParsedEpisodeInfo) (string, error) {
	if fileInfo == nil || folderInfo == nil {
		return "", nil
	}
	folderEpisodes, err := ps.GetEpisodes(ctx, folderInfo, s, true, nil)
	if err != nil || len(folderEpisodes) == 0 {
		return "", err
	}
	fileEpisodes, err := ps.GetEpisodes(ctx, fileInfo, s, true, nil)
	if err != nil {
		return "", err
	}
	var unexpected []string
	for _, ep := range fileEpisodes {
		found := false
		for _, o := range folderEpisodes {
			found = found || o.ID == ep.ID
		}
		if !found {
			unexpected = append(unexpected, slotOf(ep).String())
		}
	}
	switch len(unexpected) {
	case 0:
		return "", nil
	case 1:
		return fmt.Sprintf(msgEpisodeUnexpected, unexpected[0], folderInfo.ReleaseTitle), nil
	}
	return fmt.Sprintf(msgEpisodesUnexpected, strings.Join(unexpected, ", "), folderInfo.ReleaseTitle), nil
}

// missingEpisodeTitle returns why episodes recently aired cannot be named
// yet, as Sonarr's EpisodeTitleSpecification: when files are named with
// episode titles, an episode aired within the last 48 hours needs a
// title other than TBA, but with EpisodeTitleBulkSeasonReleases not when
// fewer than four episodes of its season aired that day. Sonarr counts
// the episodes of the season numbered as the first episode; the episodes
// of its season are counted here.
func missingEpisodeTitle(required core.EpisodeTitleRequired, n organize.SeriesNaming, s organize.Series,
	episodes []*series.Episode, all []*series.Episode, now time.Time,
) string {
	if required == core.EpisodeTitleNever || len(episodes) == 0 || !organize.RequiresEpisodeTitle(n, s, organizeEpisodes(episodes)) {
		return ""
	}
	first := episodes[0]
	sameDay := func(ep *series.Episode) bool { return ep.AirDateUTC.IsZero() || ep.AirDateUTC.Equal(first.AirDateUTC) }
	if required == core.EpisodeTitleBulkSeasonReleases && !first.AirDateUTC.IsZero() {
		allSameDay := true
		for _, ep := range episodes {
			allSameDay = allSameDay && sameDay(ep)
		}
		inSeason := 0
		for _, ep := range all {
			if ep.SeasonNumber == first.SeasonNumber && sameDay(ep) {
				inSeason++
			}
		}
		if allSameDay && inSeason < 4 {
			return ""
		}
	}
	for _, ep := range episodes {
		if !ep.AirDateUTC.IsZero() && ep.AirDateUTC.Before(now.Add(-48*time.Hour)) {
			continue
		}
		switch {
		case strings.TrimSpace(ep.Title) == "":
			return msgTitleMissing
		case ep.Title == "TBA":
			return msgTitleTBA
		}
	}
	return ""
}

// unverifiedSceneNumbering reports whether an episode's scene numbering
// is not confirmed, as Sonarr's UnverifiedSceneNumberingSpecification.
func unverifiedSceneNumbering(episodes []*series.Episode) bool {
	for _, ep := range episodes {
		if ep.UnverifiedSceneNumbering {
			return true
		}
	}
	return false
}

// missingAbsoluteNumber reports whether an anime episode aired within the
// last day has no absolute number that the anime episode format needs,
// as Sonarr's AbsoluteEpisodeNumberSpecification.
func missingAbsoluteNumber(n organize.SeriesNaming, s organize.Series, episodes []*series.Episode, now time.Time) bool {
	if s.Type != organize.SeriesAnime || !organize.RequiresAbsoluteEpisodeNumber(n) {
		return false
	}
	for _, ep := range episodes {
		if !ep.AirDateUTC.IsZero() && ep.AirDateUTC.Before(now.Add(-24*time.Hour)) {
			continue
		}
		if ep.AbsoluteEpisodeNumber == 0 {
			return true
		}
	}
	return false
}

// organizeEpisodes returns episodes as organize names them.
func organizeEpisodes(episodes []*series.Episode) []organize.Episode {
	out := make([]organize.Episode, len(episodes))
	for i, ep := range episodes {
		out[i] = organize.Episode{
			SeasonNumber: ep.SeasonNumber, EpisodeNumber: ep.EpisodeNumber, AbsoluteEpisodeNumber: ep.AbsoluteEpisodeNumber,
			AirDate: ep.AirDate, Title: ep.Title,
		}
	}
	return out
}
