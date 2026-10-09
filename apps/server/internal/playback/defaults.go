package playback

import (
	"slices"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/decision"
)

// subtitleModes maps the user's subtitle mode to the selector's; "foreign"
// behaves as "smart", without forced subtitles in the preferred language.
var subtitleModes = map[core.SubtitleMode]decision.SubtitleMode{
	core.SubtitlesDefault:    decision.SubtitlesDefault,
	core.SubtitlesAlways:     decision.SubtitlesAlways,
	core.SubtitlesForeign:    decision.SubtitlesSmart,
	core.SubtitlesForcedOnly: decision.SubtitlesOnlyForced,
	core.SubtitlesNone:       decision.SubtitlesNone,
	core.SubtitlesSmart:      decision.SubtitlesSmart,
}

// sourceFor offers a media source with the streams the user plays by
// default: the ones they last chose, else those their preferences pick,
// as Jellyfin does.
func sourceFor(ms *core.MediaSource, prefs *core.UserPreferences, data *core.UserData) *decision.Source {
	s := &decision.Source{MediaSource: ms}
	if i := data.AudioStream; i != nil && hasStream(ms, core.StreamAudio, *i) {
		s.DefaultAudioIndex, s.AudioIndexSource = new(*i), decision.AudioIndexUser
	} else {
		s.DefaultAudioIndex = decision.DefaultAudioIndex(ms.Streams, prefs.AudioLanguages, prefs.PlayDefaultAudioTrack)
		if prefs.PlayDefaultAudioTrack {
			s.AudioIndexSource |= decision.AudioIndexDefault
		}
		if len(prefs.AudioLanguages) > 0 {
			s.AudioIndexSource |= decision.AudioIndexLanguage
		}
	}

	mode := subtitleModes[prefs.SubtitleMode]
	if i := data.SubtitleStream; i != nil && mode != decision.SubtitlesNone {
		// "Only forced" rules out remembered full subtitles.
		st := stream(ms, core.StreamSubtitle, *i)
		if *i == -1 || (st != nil && (mode != decision.SubtitlesOnlyForced || st.Forced)) {
			s.DefaultSubtitleIndex = new(*i)
			return s
		}
	}
	var audioLanguage string
	if s.DefaultAudioIndex != nil {
		if st := stream(ms, core.StreamAudio, *s.DefaultAudioIndex); st != nil {
			audioLanguage = st.Language
		}
	}
	s.DefaultSubtitleIndex = decision.DefaultSubtitleIndex(ms.Streams, prefs.SubtitleLanguages, mode, audioLanguage)
	s.SubtitleScores = decision.SubtitleScores(ms.Streams, prefs.SubtitleLanguages, mode, audioLanguage)
	return s
}

func stream(ms *core.MediaSource, kind core.StreamKind, index int) *core.MediaStream {
	i := slices.IndexFunc(ms.Streams, func(st core.MediaStream) bool { return st.Kind == kind && st.Index == index })
	if i < 0 {
		return nil
	}
	return &ms.Streams[i]
}

func hasStream(ms *core.MediaSource, kind core.StreamKind, index int) bool {
	return stream(ms, kind, index) != nil
}
