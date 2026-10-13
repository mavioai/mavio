package core

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
)

// SubtitleLanguage is a language of subtitles with their kind, as Bazarr's
// "en", "en:hi" and "en:forced".
type SubtitleLanguage struct {
	// Language is a BCP 47 tag, such as "en", "pt-BR" or "zh-Hant".
	Language        string
	Forced          bool
	HearingImpaired bool
}

// String returns the language as Bazarr writes it: the tag, followed by
// ":forced" or ":hi".
func (l SubtitleLanguage) String() string {
	switch {
	case l.Forced:
		return l.Language + ":forced"
	case l.HearingImpaired:
		return l.Language + ":hi"
	}
	return l.Language
}

// Validate checks that the language is a tag, and not both forced and
// hearing-impaired.
func (l SubtitleLanguage) Validate() error {
	switch {
	case !validLanguageTag(l.Language):
		return fmt.Errorf("%w: subtitle language %q is not a language tag", ErrInvalid, l.Language)
	case l.Forced && l.HearingImpaired:
		return fmt.Errorf("%w: subtitles in %s both forced and hearing-impaired", ErrInvalid, l.Language)
	}
	return nil
}

// validLanguageTag reports whether s looks like a BCP 47 tag: subtags of
// letters and digits separated by hyphens, the first of letters.
func validLanguageTag(s string) bool {
	if s == "" {
		return false
	}
	for i, sub := range strings.Split(s, "-") {
		if sub == "" || len(sub) > 8 {
			return false
		}
		for _, r := range sub {
			letter := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
			if !letter && (i == 0 || r < '0' || r > '9') {
				return false
			}
		}
	}
	return true
}

// SubtitleHI is how a subtitle profile item treats hearing-impaired
// subtitles.
type SubtitleHI string

// The hearing-impaired modes.
const (
	// SubtitleHIAllowed wants normal subtitles and takes hearing-impaired
	// ones in their place.
	SubtitleHIAllowed SubtitleHI = ""
	// SubtitleHIRequired wants hearing-impaired subtitles.
	SubtitleHIRequired SubtitleHI = "required"
	// SubtitleHIExcluded wants normal subtitles only.
	SubtitleHIExcluded SubtitleHI = "excluded"
)

// Valid reports whether the mode is known.
func (h SubtitleHI) Valid() bool {
	return h == SubtitleHIAllowed || h == SubtitleHIRequired || h == SubtitleHIExcluded
}

// SubtitleCutoffAny is the cutoff of a subtitle profile satisfied by any
// of its languages.
const SubtitleCutoffAny = 65535

// SubtitleProfile is the subtitles movies and episodes want, as Bazarr's
// language profiles.
type SubtitleProfile struct {
	ID    ID
	Name  string
	Items []SubtitleProfileItem
	// Cutoff is the ID of the item whose subtitles end the search for the
	// others, SubtitleCutoffAny for any item's, or 0 for none.
	Cutoff int
	// MustContain and MustNotContain are words subtitle release names must
	// and must not contain.
	MustContain, MustNotContain []string
	// OriginalFormat keeps subtitles in the format their provider gives
	// instead of converting them to SubRip.
	OriginalFormat bool
	// Tag assigns the profile to movies and series with the tag, when the
	// settings allow it.
	Tag string
}

// SubtitleProfileItem is a language a subtitle profile wants.
type SubtitleProfileItem struct {
	// ID identifies the item in its profile, from 1.
	ID       int
	Language string
	Forced   bool
	HI       SubtitleHI
	// AudioExclude does not want the subtitles when an audio stream has
	// the language; AudioOnlyInclude wants them only then.
	AudioExclude, AudioOnlyInclude bool
}

// Validate checks the profile's invariants.
func (p *SubtitleProfile) Validate() error {
	switch {
	case p.ID.IsZero():
		return fmt.Errorf("%w: subtitle profile requires an ID", ErrInvalid)
	case strings.TrimSpace(p.Name) == "":
		return fmt.Errorf("%w: subtitle profile requires a name", ErrInvalid)
	case len(p.Items) == 0:
		return fmt.Errorf("%w: subtitle profile %q has no languages", ErrInvalid, p.Name)
	case strings.ContainsFunc(p.Tag, isSpace):
		return fmt.Errorf("%w: subtitle profile %q has tag %q", ErrInvalid, p.Name, p.Tag)
	}
	ids := map[int]bool{}
	langs := map[SubtitleProfileItem]bool{}
	for _, it := range p.Items {
		key := SubtitleProfileItem{Language: it.Language, Forced: it.Forced, HI: it.HI}
		switch {
		case it.ID <= 0 || it.ID >= SubtitleCutoffAny || ids[it.ID]:
			return fmt.Errorf("%w: subtitle profile %q has item ID %d twice or out of range", ErrInvalid, p.Name, it.ID)
		case !validLanguageTag(it.Language):
			return fmt.Errorf("%w: subtitle profile %q has language %q", ErrInvalid, p.Name, it.Language)
		case !it.HI.Valid():
			return fmt.Errorf("%w: subtitle profile %q has hearing-impaired mode %q", ErrInvalid, p.Name, it.HI)
		case it.Forced && it.HI == SubtitleHIRequired:
			return fmt.Errorf("%w: subtitle profile %q wants %s both forced and hearing-impaired", ErrInvalid, p.Name, it.Language)
		case it.AudioExclude && it.AudioOnlyInclude:
			return fmt.Errorf("%w: subtitle profile %q both excludes and only includes %s by audio", ErrInvalid, p.Name, it.Language)
		case langs[key]:
			return fmt.Errorf("%w: subtitle profile %q wants %s twice", ErrInvalid, p.Name, it.Language)
		}
		ids[it.ID], langs[key] = true, true
	}
	if p.Cutoff != 0 && p.Cutoff != SubtitleCutoffAny && !ids[p.Cutoff] {
		return fmt.Errorf("%w: subtitle profile %q has cutoff %d, not one of its items", ErrInvalid, p.Name, p.Cutoff)
	}
	return nil
}

func isSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }

// SubtitleProfileAssignment assigns a subtitle profile to a library, a
// movie or a series. As in Bazarr, where the settings let tags assign
// profiles, a movie's or series' tags take precedence over its own
// assignment, which takes precedence over its library's.
type SubtitleProfileAssignment struct {
	// TargetID is the library, movie or series.
	TargetID ID
	// ProfileID is the profile, or zero for none: no subtitles are wanted.
	ProfileID ID
}

// SubtitleAction is what happened to a subtitle.
type SubtitleAction string

// The subtitle actions, as Bazarr's history actions.
const (
	SubtitleDeleted    SubtitleAction = "deleted"
	SubtitleDownloaded SubtitleAction = "downloaded"
	// SubtitleManual is a subtitle downloaded from a manual search.
	SubtitleManual    SubtitleAction = "manual"
	SubtitleUpgraded  SubtitleAction = "upgraded"
	SubtitleUploaded  SubtitleAction = "uploaded"
	SubtitleSynced    SubtitleAction = "synced"
	SubtitleExtracted SubtitleAction = "extracted"
)

// SubtitleActions lists the subtitle actions.
var SubtitleActions = []SubtitleAction{
	SubtitleDeleted, SubtitleDownloaded, SubtitleManual, SubtitleUpgraded, SubtitleUploaded, SubtitleSynced, SubtitleExtracted,
}

// Valid reports whether the action is known.
func (a SubtitleAction) Valid() bool { return slices.Contains(SubtitleActions, a) }

// SubtitleHistoryEvent is something that happened to a subtitle of a movie
// or an episode.
type SubtitleHistoryEvent struct {
	ID     ID
	Action SubtitleAction
	// ItemID is the movie or episode; ParentID the series of an episode.
	ItemID, ParentID ID
	Date             time.Time
	Language         SubtitleLanguage
	// Provider is the plugin ID and provider name of a subtitle
	// downloaded, as "subtitles/opensubtitlescom".
	Provider string
	// SubtitleID is the provider's ID of the subtitle.
	SubtitleID string
	// Score and MaxScore are the subtitle's score and the most it could
	// have; MaxScore is 0 when the subtitle was not scored.
	Score, MaxScore int
	// Path is the subtitle file; VideoPath the video's.
	Path, VideoPath string
	// Description says what happened, as Bazarr's messages.
	Description string
	// Matched and NotMatched are the properties of the video the
	// subtitle matched and did not.
	Matched, NotMatched []string
	// UpgradedFrom is the event of the download an upgrade replaced.
	UpgradedFrom ID
}

// Validate checks the event's invariants.
func (h *SubtitleHistoryEvent) Validate() error {
	switch {
	case h.ID.IsZero() || h.ItemID.IsZero() || h.Date.IsZero():
		return fmt.Errorf("%w: subtitle history event requires an ID, an item and a date", ErrInvalid)
	case !h.Action.Valid():
		return fmt.Errorf("%w: subtitle history event of action %q", ErrInvalid, h.Action)
	case h.Score < 0 || h.MaxScore < 0 || h.MaxScore > 0 && h.Score > h.MaxScore:
		return fmt.Errorf("%w: subtitle history event scored %d of %d", ErrInvalid, h.Score, h.MaxScore)
	case h.UpgradedFrom == h.ID:
		return fmt.Errorf("%w: subtitle history event %s upgrades itself", ErrInvalid, h.ID)
	}
	return h.Language.Validate()
}

// SubtitleHistoryQuery selects subtitle history events, newest first; zero
// values do not filter.
type SubtitleHistoryQuery struct {
	// ItemIDs select events of these items, or of episodes of these
	// series.
	ItemIDs []ID
	Actions []SubtitleAction
	Since   time.Time
	Limit   int
	Offset  int
}

// SubtitleBlocklistEntry is a subtitle not to download again for a movie
// or an episode.
type SubtitleBlocklistEntry struct {
	ID ID
	// ItemID is the movie or episode; ParentID the series of an episode.
	ItemID, ParentID ID
	Provider         string
	SubtitleID       string
	Language         SubtitleLanguage
	Date             time.Time
	// Message is why the subtitle was blocklisted.
	Message string
}

// Validate checks the entry's invariants.
func (b *SubtitleBlocklistEntry) Validate() error {
	switch {
	case b.ID.IsZero() || b.ItemID.IsZero() || b.Date.IsZero():
		return fmt.Errorf("%w: subtitle blocklist entry requires an ID, an item and a date", ErrInvalid)
	case b.Provider == "" || b.SubtitleID == "":
		return fmt.Errorf("%w: subtitle blocklist entry %s requires a provider and a subtitle ID", ErrInvalid, b.ID)
	}
	return b.Language.Validate()
}

// SubtitleSearchAttempt records the failed searches of a movie's or
// episode's subtitles in a language, as Bazarr's failedAttempts: the
// first and the latest.
type SubtitleSearchAttempt struct {
	ItemID      ID
	Language    SubtitleLanguage
	First, Last time.Time
}

// SubtitleFolder is where subtitles are saved, as Bazarr's subfolder
// setting.
type SubtitleFolder string

// The subtitle folders.
const (
	// SubtitleFolderCurrent saves subtitles beside the video.
	SubtitleFolderCurrent SubtitleFolder = "current"
	// SubtitleFolderRelative saves them in a folder of the video's
	// folder, named SubtitleSettings.CustomFolder.
	SubtitleFolderRelative SubtitleFolder = "relative"
	// SubtitleFolderMetadata saves them in the item's folder within the
	// server's metadata folder, where artwork of libraries that do not
	// save local metadata is kept too; it takes the place of Bazarr's
	// absolute folder.
	SubtitleFolderMetadata SubtitleFolder = "metadata"
)

// SubtitleLanguageEquivalence counts subtitles in one language as
// subtitles in another, as Bazarr's language_equals.
type SubtitleLanguageEquivalence struct {
	From, To SubtitleLanguage
}

// SubtitleSettings are how subtitles are searched, chosen, saved and
// synchronized, as Bazarr's general and subsync settings.
type SubtitleSettings struct {
	// MinimumScore and MinimumScoreMovie are the percentages of the
	// maximum score episodes' and movies' subtitles need.
	MinimumScore, MinimumScoreMovie int
	// UseSceneName describes videos by the release names their files came
	// from.
	UseSceneName bool
	// UseEmbedded counts embedded subtitle streams as present, but not
	// those of the formats ignored.
	UseEmbedded                        bool
	IgnorePGS, IgnoreVobSub, IgnoreASS bool
	// UTF8 saves subtitles in UTF-8.
	UTF8 bool
	// Mods are the names of the subtitle mods applied to downloads.
	Mods []string
	// HIExtension is the suffix of hearing-impaired subtitles' names:
	// "hi", "cc" or "sdh".
	HIExtension string
	// SingleLanguage saves subtitles without their language in their
	// names.
	SingleLanguage bool
	Folder         SubtitleFolder
	CustomFolder   string
	// Upgrade searches again for the subtitles downloaded in the last
	// UpgradeDays (0–30) below their maximum score, every UpgradeInterval;
	// UpgradeManual includes those downloaded by hand.
	Upgrade         bool
	UpgradeInterval time.Duration
	UpgradeDays     int
	UpgradeManual   bool
	// AdaptiveSearching searches a language that has failed, after
	// AdaptiveDelay since the first failure, only once per AdaptiveDelta.
	AdaptiveSearching            bool
	AdaptiveDelay, AdaptiveDelta time.Duration
	// WantedSearchInterval and WantedSearchIntervalMovie are how often
	// episodes' and movies' wanted subtitles are searched; 0 never.
	WantedSearchInterval, WantedSearchIntervalMovie time.Duration
	// DefaultAudioLanguage and DefaultEmbeddedLanguage are the languages
	// of audio and embedded subtitle streams without one; empty leaves
	// them unknown.
	DefaultAudioLanguage, DefaultEmbeddedLanguage string
	// SkipHashing does not compute videos' hashes for providers.
	SkipHashing    bool
	LanguageEquals []SubtitleLanguageEquivalence
	// MovieProfileTags and SeriesProfileTags let tags assign movies and
	// series subtitle profiles: that of the first profile, by name, whose
	// tag they have, or none when they have one of NoProfileTags.
	MovieProfileTags, SeriesProfileTags bool
	NoProfileTags                       []string
	Sync                                SubtitleSyncSettings
}

// SubtitleSyncSettings are how downloaded subtitles are synchronized, as
// Bazarr's subsync settings.
type SubtitleSyncSettings struct {
	// Enabled synchronizes downloaded subtitles; with UseThreshold or
	// UseMovieThreshold only episodes' below Threshold or movies' below
	// MovieThreshold, percentages of the maximum score.
	Enabled                         bool
	UseThreshold, UseMovieThreshold bool
	Threshold, MovieThreshold       int
	// ForceAudio synchronizes against the audio even when another
	// subtitle could serve as reference.
	ForceAudio bool
	// OriginalLanguage synchronizes against the audio stream in the
	// item's original language when there is one.
	OriginalLanguage bool
	// ExcludedProviders and ExcludedLanguages are the providers and
	// languages whose subtitles are not synchronized.
	ExcludedProviders, ExcludedLanguages []string
	// MaxOffset bounds the offsets searched; FixFramerate also tries the
	// common frame rate ratios; GoldenSection searches for the best ratio.
	MaxOffset     time.Duration
	FixFramerate  bool
	GoldenSection bool
	// MinScore, MaxQualityOffset and MaxScaleDeviation reject alignments
	// when FixFramerate is set.
	MinScore          float64
	MaxQualityOffset  time.Duration
	MaxScaleDeviation float64
}

// DefaultSubtitleSettings are Bazarr's defaults.
func DefaultSubtitleSettings() SubtitleSettings {
	return SubtitleSettings{
		MinimumScore:              90,
		MinimumScoreMovie:         70,
		UseSceneName:              true,
		UseEmbedded:               true,
		UTF8:                      true,
		HIExtension:               "hi",
		Folder:                    SubtitleFolderCurrent,
		Upgrade:                   true,
		UpgradeInterval:           12 * time.Hour,
		UpgradeDays:               7,
		UpgradeManual:             true,
		AdaptiveSearching:         true,
		AdaptiveDelay:             3 * 7 * 24 * time.Hour,
		AdaptiveDelta:             7 * 24 * time.Hour,
		WantedSearchInterval:      6 * time.Hour,
		WantedSearchIntervalMovie: 6 * time.Hour,
		Sync: SubtitleSyncSettings{
			Threshold:         90,
			MovieThreshold:    70,
			MaxOffset:         60 * time.Second,
			GoldenSection:     true,
			MaxQualityOffset:  30 * time.Second,
			MaxScaleDeviation: 0.1,
		},
	}
}

// Validate checks the settings' values and ranges.
func (s *SubtitleSettings) Validate() error {
	percent := func(v int) bool { return v >= 0 && v <= 100 }
	interval := func(d time.Duration) bool { return d == 0 || d >= time.Hour }
	switch {
	case !percent(s.MinimumScore) || !percent(s.MinimumScoreMovie):
		return fmt.Errorf("%w: subtitle minimum score outside 0–100", ErrInvalid)
	case !slices.Contains([]string{"hi", "cc", "sdh"}, s.HIExtension):
		return fmt.Errorf("%w: hearing-impaired extension %q", ErrInvalid, s.HIExtension)
	case s.Folder != SubtitleFolderCurrent && s.Folder != SubtitleFolderRelative && s.Folder != SubtitleFolderMetadata:
		return fmt.Errorf("%w: subtitle folder %q", ErrInvalid, s.Folder)
	case s.Folder == SubtitleFolderRelative && (s.CustomFolder == "" || s.CustomFolder == "." || s.CustomFolder == ".." ||
		strings.ContainsAny(s.CustomFolder, `/\:`)):
		return fmt.Errorf("%w: relative subtitle folder %q", ErrInvalid, s.CustomFolder)
	case s.Folder != SubtitleFolderRelative && s.CustomFolder != "":
		return fmt.Errorf("%w: a custom subtitle folder without the relative folder setting", ErrInvalid)
	case s.UpgradeDays < 0 || s.UpgradeDays > 30:
		return fmt.Errorf("%w: subtitle upgrade days outside 0–30", ErrInvalid)
	case !interval(s.UpgradeInterval) || !interval(s.WantedSearchInterval) || !interval(s.WantedSearchIntervalMovie):
		return fmt.Errorf("%w: subtitle search interval under an hour", ErrInvalid)
	case s.AdaptiveDelay < 0 || s.AdaptiveDelta < 0:
		return fmt.Errorf("%w: negative adaptive searching interval", ErrInvalid)
	case s.DefaultAudioLanguage != "" && !validLanguageTag(s.DefaultAudioLanguage),
		s.DefaultEmbeddedLanguage != "" && !validLanguageTag(s.DefaultEmbeddedLanguage):
		return fmt.Errorf("%w: default subtitle or audio language is not a language tag", ErrInvalid)
	case slices.Contains(s.Mods, ""):
		return fmt.Errorf("%w: empty subtitle mod", ErrInvalid)
	case slices.ContainsFunc(s.NoProfileTags, func(t string) bool { return t == "" || strings.ContainsFunc(t, isSpace) }):
		return fmt.Errorf("%w: subtitle profile tags must be words", ErrInvalid)
	}
	for _, e := range s.LanguageEquals {
		if err := e.From.Validate(); err != nil {
			return err
		}
		if err := e.To.Validate(); err != nil {
			return err
		}
	}
	y := &s.Sync
	switch {
	case !percent(y.Threshold) || !percent(y.MovieThreshold):
		return fmt.Errorf("%w: synchronization threshold outside 0–100", ErrInvalid)
	case y.MaxOffset <= 0 || y.MaxQualityOffset < 0:
		return fmt.Errorf("%w: synchronization offsets must be positive", ErrInvalid)
	case y.MinScore < 0 || y.MaxScaleDeviation < 0:
		return fmt.Errorf("%w: negative synchronization quality bound", ErrInvalid)
	}
	return nil
}

// Repositories of subtitle acquisition.
type (
	// SubtitleProfileRepository stores subtitle profiles. Names are
	// unique (ErrConflict); deleting a profile deletes its assignments.
	SubtitleProfileRepository interface {
		Get(ctx context.Context, id ID) (SubtitleProfile, error)
		// List returns the profiles by name.
		List(ctx context.Context) ([]SubtitleProfile, error)
		Put(ctx context.Context, p *SubtitleProfile) error
		Delete(ctx context.Context, id ID) error
	}

	// SubtitleProfileAssignmentRepository stores the assignments of
	// subtitle profiles to libraries, movies and series; deleting the
	// target deletes its assignment. Put fails with ErrNotFound when the
	// target or the profile does not exist.
	SubtitleProfileAssignmentRepository interface {
		// Get returns ErrNotFound for a target without an assignment.
		Get(ctx context.Context, targetID ID) (SubtitleProfileAssignment, error)
		// List returns the assignments of targets, or all when none are
		// given.
		List(ctx context.Context, targetIDs ...ID) ([]SubtitleProfileAssignment, error)
		Put(ctx context.Context, a SubtitleProfileAssignment) error
		Delete(ctx context.Context, targetID ID) error
	}

	// SubtitleHistoryRepository stores subtitle history; deleting an item
	// deletes its events.
	SubtitleHistoryRepository interface {
		Add(ctx context.Context, events ...SubtitleHistoryEvent) error
		// List returns the events q selects, newest first.
		List(ctx context.Context, q SubtitleHistoryQuery) (Page[SubtitleHistoryEvent], error)
		// Purge deletes the events before a time and reports how many.
		Purge(ctx context.Context, before time.Time) (int, error)
	}

	// SubtitleBlocklistRepository stores blocklisted subtitles; deleting
	// the item deletes its entries.
	SubtitleBlocklistRepository interface {
		Add(ctx context.Context, b *SubtitleBlocklistEntry) error
		// List returns the entries of an item, or all when itemID is zero,
		// newest first.
		List(ctx context.Context, itemID ID, limit, offset int) (Page[SubtitleBlocklistEntry], error)
		Delete(ctx context.Context, ids ...ID) error
		// Clear deletes every entry.
		Clear(ctx context.Context) error
	}

	// SubtitleSearchAttemptRepository stores failed subtitle searches;
	// deleting the item deletes them.
	SubtitleSearchAttemptRepository interface {
		// List returns the attempts of an item by language.
		List(ctx context.Context, itemID ID) ([]SubtitleSearchAttempt, error)
		// Record records a failed search of an item's subtitles in a
		// language at a time: the first time is kept, the latest replaced.
		Record(ctx context.Context, itemID ID, lang SubtitleLanguage, at time.Time) error
	}
)
