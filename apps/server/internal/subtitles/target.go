package subtitles

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"golang.org/x/text/language"

	"github.com/mavioai/mavio/libs/acquire/subtitles"
	"github.com/mavioai/mavio/libs/core"
)

// target is a movie or an episode, with what its subtitles are searched
// and counted by.
type target struct {
	item core.Item
	lib  core.Library
	// owner is what profiles are assigned to: the movie, or the
	// episode's series.
	owner   core.Item
	series  *core.Item
	sources []core.MediaSource
	// si is the video file's source.
	si      int
	profile *core.SubtitleProfile
	// assigned is the profile assigned to the owner, or else its library.
	assigned core.ID
	// subs are the subtitle streams, with whether they count as present.
	subs  []subtitleStream
	audio []string
	video *subtitles.Video
	// scene is the release name the file was imported from.
	scene string
}

// subtitleStream is a subtitle stream of a video.
type subtitleStream struct {
	stream   core.MediaStream
	language core.SubtitleLanguage
	// counted is set for the streams that provide their language.
	counted bool
}

func (t *target) source() *core.MediaSource { return &t.sources[t.si] }

func (t *target) episode() bool { return t.item.Kind == core.KindEpisode }

// existing returns the languages the counted subtitles provide.
func (t *target) existing() []core.SubtitleLanguage {
	var out []core.SubtitleLanguage
	for _, s := range t.subs {
		if s.counted {
			out = append(out, s.language)
		}
	}
	return out
}

// missing returns the subtitles the target's profile still wants.
func (t *target) missing(s *core.SubtitleSettings) []core.SubtitleLanguage {
	if t.profile == nil {
		return nil
	}
	return subtitles.Missing(t.profile, t.existing(), t.audio, subtitles.LanguageEquals(s.LanguageEquals))
}

// errNoSubtitles is the error of items that have no subtitles of their
// own, such as series.
var errNoSubtitles = fmt.Errorf("%w: only movies and episodes have subtitles", core.ErrInvalid)

// load returns a movie or an episode as a target.
func (r *Runtime) load(ctx context.Context, itemID core.ID, s *core.SubtitleSettings) (*target, error) {
	it, err := r.Store.Items().Get(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if it.Kind != core.KindMovie && it.Kind != core.KindEpisode {
		return nil, errNoSubtitles
	}
	t := &target{item: it, owner: it}
	if t.lib, err = r.Store.Libraries().Get(ctx, it.LibraryID); err != nil {
		return nil, err
	}
	if it.Kind == core.KindEpisode {
		for id := it.ParentID; !id.IsZero(); {
			p, err := r.Store.Items().Get(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("series of episode %s: %w", it.ID, err)
			}
			if p.Kind == core.KindSeries {
				t.series, t.owner = &p, p
				break
			}
			id = p.ParentID
		}
	}
	if t.sources, err = r.Store.MediaSources().ListForItem(ctx, it.ID); err != nil {
		return nil, err
	}
	t.si = slices.IndexFunc(t.sources, func(src core.MediaSource) bool { return src.Disc == "" && src.Path != "" })
	if t.si < 0 {
		return nil, fmt.Errorf("%w: %s has no video file", core.ErrNotFound, it.Name)
	}
	if err := r.resolveProfile(ctx, t, s); err != nil {
		return nil, err
	}
	t.subs, t.audio = streamsOf(t.source().Streams, s, t.singleLanguage(s))
	if s.UseSceneName {
		page, err := r.Store.History().List(ctx, core.HistoryQuery{ItemIDs: []core.ID{it.ID}, Types: []core.HistoryEventType{core.HistoryImported}, Limit: 1})
		if err != nil {
			return nil, err
		}
		if len(page.Items) > 0 {
			t.scene = page.Items[0].SourceTitle
		}
	}
	t.video = t.describe()
	return t, nil
}

// resolveProfile sets the target's profile: that assigned to the movie or
// series, or else to its library, overridden by tags as the settings say.
func (r *Runtime) resolveProfile(ctx context.Context, t *target, s *core.SubtitleSettings) error {
	for _, id := range []core.ID{t.owner.ID, t.lib.ID} {
		a, err := r.Store.SubtitleProfileAssignments().Get(ctx, id)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		t.assigned = a.ProfileID
		break
	}
	profiles, err := r.Store.SubtitleProfiles().List(ctx)
	if err != nil {
		return err
	}
	useTags := s.MovieProfileTags
	if t.series != nil {
		useTags = s.SeriesProfileTags
	}
	t.profile = subtitles.ProfileFor(profiles, t.assigned, t.owner.Tags, useTags, s.NoProfileTags)
	return nil
}

// singleLanguage returns the language of subtitle files named without
// one when the settings save subtitles so: the first of the profile's.
func (t *target) singleLanguage(s *core.SubtitleSettings) string {
	if !s.SingleLanguage || t.profile == nil || len(t.profile.Items) == 0 {
		return ""
	}
	return t.profile.Items[0].Language
}

// Codecs of embedded subtitles the settings can ignore, as ffprobe names
// them.
const (
	codecPGS    = "hdmv_pgs_subtitle"
	codecVobSub = "dvd_subtitle"
)

// streamsOf returns a video's subtitle streams and its audio languages,
// as Bazarr's embedded subtitle and audio readers: subtitle files always
// count, embedded subtitles when the settings use them and do not ignore
// their format. Streams without a language take the settings' default
// language, files single, or are left out.
func streamsOf(streams []core.MediaStream, s *core.SubtitleSettings, single string) ([]subtitleStream, []string) {
	var subs []subtitleStream
	var audio []string
	for _, st := range streams {
		switch st.Kind {
		case core.StreamAudio:
			lang := Tag(st.Language)
			if lang == "" {
				lang = s.DefaultAudioLanguage
			}
			if lang != "" && !slices.Contains(audio, lang) {
				audio = append(audio, lang)
			}
		case core.StreamSubtitle:
			lang := Tag(st.Language)
			external := st.ExternalPath != ""
			switch {
			case lang != "":
			case external:
				lang = single
			default:
				lang = s.DefaultEmbeddedLanguage
			}
			if lang == "" {
				continue
			}
			counted := external
			if !external && s.UseEmbedded {
				codec := strings.ToLower(st.Codec)
				ignored := s.IgnorePGS && codec == codecPGS || s.IgnoreVobSub && codec == codecVobSub ||
					s.IgnoreASS && (codec == "ass" || codec == "ssa")
				counted = !ignored
			}
			subs = append(subs, subtitleStream{
				stream: st, counted: counted,
				language: core.SubtitleLanguage{Language: lang, Forced: st.Forced, HearingImpaired: st.HearingImpaired && !st.Forced},
			})
		}
	}
	return subs, audio
}

// Tag returns a language as a BCP 47 tag, from the ISO 639 codes and tags
// streams record ("fre" is "fr", "zh-Hans" stays), or "" for none or an
// undetermined language.
func Tag(code string) string {
	if code == "" {
		return ""
	}
	tag, err := language.Parse(code)
	if err != nil || tag == language.Und {
		return ""
	}
	return tag.String()
}

// describe returns the video subtitles are scored against: the item's
// titles and numbers, its file's streams, and what its release name and
// file name tell.
func (t *target) describe() *subtitles.Video {
	it := &t.item
	v := &subtitles.Video{Episode: t.episode(), Title: it.Name, Year: it.ProductionYear}
	if t.episode() {
		if it.ParentIndexNumber != nil {
			v.Season = *it.ParentIndexNumber
			v.Special = v.Season == 0
		}
		if it.IndexNumber != nil {
			v.EpisodeNumber = *it.IndexNumber
		}
		if t.series != nil {
			v.Series, v.Year = t.series.Name, t.series.ProductionYear
			v.AlternativeSeries = t.series.AlternativeTitles
			v.OriginalSeries = !strings.Contains(t.series.Name, "(")
		}
	} else {
		v.AlternativeTitles = it.AlternativeTitles
	}
	for _, st := range t.source().Streams {
		switch {
		case st.Kind == core.StreamVideo && v.VideoCodec == "":
			v.VideoCodec = subtitles.VideoCodecOf(st.Codec)
			v.Resolution = resolutionOf(st.Width, st.Height)
		case st.Kind == core.StreamAudio && v.AudioCodec == "" && st.ExternalPath == "":
			v.AudioCodec = subtitles.AudioCodecOf(st.Codec, st.Profile)
		}
	}
	if t.scene != "" {
		v.Refine(subtitles.ParseGuess(t.scene, v.Episode))
	}
	name := path.Base(t.source().Path)
	v.Refine(subtitles.ParseGuess(strings.TrimSuffix(name, path.Ext(name)), v.Episode))
	return v
}

// resolutionOf names a video's resolution as guessit does, by its height
// or, for wide videos cropped in height, its width.
func resolutionOf(width, height int) string {
	lines := height
	switch {
	case width >= 3800:
		lines = max(lines, 2160)
	case width >= 1900:
		lines = max(lines, 1080)
	case width >= 1260:
		lines = max(lines, 720)
	}
	switch {
	case lines >= 2000:
		lines = 2160
	case lines >= 1000:
		lines = 1080
	case lines >= 700:
		lines = 720
	case lines >= 560:
		lines = 576
	case lines >= 470:
		lines = 480
	}
	return subtitles.ResolutionName(lines)
}
