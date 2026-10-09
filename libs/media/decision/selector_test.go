package decision

import (
	"encoding/json"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

func TestMediaStreamSelectorCases(t *testing.T) {
	const derived = "SupportsExternalStream is derived from the stream (external, text, PGS or VobSub) rather than set on its own"
	portedCases(t, "media_stream_selector.json", ported{
		run: map[string]func(t *testing.T, a args){
			"GetDefaultAudioStreamIndex_EmptyStreams_Null": func(t *testing.T, a args) {
				if got := DefaultAudioIndex(nil, nil, a.boolean(t, "preferDefaultTrack")); got != nil {
					t.Errorf("got = %d, want = nil", *got)
				}
			},
			"GetDefaultAudioStreamIndex_PreferredLanguage_SelectsCorrect": func(t *testing.T, a args) {
				streams := []core.MediaStream{
					{Index: 0, Kind: core.StreamVideo, Default: true},
					{Index: 1, Kind: core.StreamAudio, Language: "fre", Default: true},
					{Index: 2, Kind: core.StreamAudio, Language: "eng"},
				}
				got := DefaultAudioIndex(streams, a.strs(t, "preferredLanguages"), a.boolean(t, "preferDefaultTrack"))
				if want := a.integer(t, "expectedIndex"); got == nil || *got != want {
					t.Errorf("got = %v, want = %d", got, want)
				}
			},
			"GetStreamScore_MediaStream_CorrectScore": func(t *testing.T, a args) {
				var arg struct {
					Init struct {
						Language                        string
						IsForced, IsDefault, IsExternal bool
						SupportsExternalStream          bool
					}
				}
				if err := json.Unmarshal(a["stream"], &arg); err != nil {
					t.Fatal(err)
				}
				in := arg.Init
				// Jellyfin's default stream type is audio.
				st := core.MediaStream{Kind: core.StreamAudio, Language: in.Language, Forced: in.IsForced, Default: in.IsDefault}
				if in.IsExternal {
					st.ExternalPath = "external"
				}
				if supportsExternalStream(&st) != in.SupportsExternalStream {
					t.Skip(derived)
				}
				if got, want := StreamScore(&st, []string{"eng", "fre"}), a.integer(t, "expectedScore"); got != want {
					t.Errorf("got = %d, want = %d", got, want)
				}
			},
		},
	})
}

func TestDefaultSubtitleIndex(t *testing.T) {
	streams := []core.MediaStream{
		{Index: 0, Kind: core.StreamVideo},
		{Index: 1, Kind: core.StreamAudio, Language: "jpn"},
		{Index: 2, Kind: core.StreamSubtitle, Codec: "subrip", Language: "eng"},
		{Index: 3, Kind: core.StreamSubtitle, Codec: "subrip", Language: "eng", Forced: true},
		{Index: 4, Kind: core.StreamSubtitle, Codec: "subrip", Language: "und", Forced: true},
		{Index: 5, Kind: core.StreamSubtitle, Codec: "ass", Language: "fre", Default: true},
	}
	eng := []string{"eng"}
	tests := []struct {
		mode  SubtitleMode
		audio string
		want  int // -1 for none
	}{
		{SubtitlesNone, "jpn", -1},
		{SubtitlesDefault, "jpn", 5},
		{SubtitlesAlways, "jpn", 2},
		{SubtitlesOnlyForced, "jpn", 3},
		{SubtitlesSmart, "jpn", 2},
		{SubtitlesSmart, "eng", 3},
	}
	for _, tt := range tests {
		got := DefaultSubtitleIndex(streams, eng, tt.mode, tt.audio)
		if (got == nil) != (tt.want == -1) || (got != nil && *got != tt.want) {
			t.Errorf("%s/%s: got = %v, want = %d", tt.mode, tt.audio, got, tt.want)
		}
	}
	if s := SubtitleScores(streams, eng, SubtitlesAlways, "jpn"); len(s) != 1 || s[2] == 0 {
		t.Errorf("scores: got = %v, want only stream 2", s)
	}
	if s := SubtitleScores(streams[:2], eng, SubtitlesAlways, "jpn"); len(s) != 0 {
		t.Errorf("scores without subtitles: got = %v", s)
	}
}

func TestChineseSubtitleSelection(t *testing.T) {
	streams := []core.MediaStream{
		{Index: 0, Kind: core.StreamVideo},
		{Index: 1, Kind: core.StreamAudio, Language: "jpn"},
		{Index: 2, Kind: core.StreamSubtitle, Codec: "subrip", Language: "chs"},
		{Index: 3, Kind: core.StreamSubtitle, Codec: "ass", Language: "cht"},
		{Index: 4, Kind: core.StreamSubtitle, Codec: "subrip", Language: "eng"},
	}

	// Preference "zh-Hans" should match stream 2 ("chs")
	gotHans := DefaultSubtitleIndex(streams, []string{"zh-Hans"}, SubtitlesAlways, "jpn")
	if gotHans == nil || *gotHans != 2 {
		t.Errorf("zh-Hans preferred: got = %v, want = 2", gotHans)
	}

	// Preference "zh-Hant" should match stream 3 ("cht")
	gotHant := DefaultSubtitleIndex(streams, []string{"zh-Hant"}, SubtitlesAlways, "jpn")
	if gotHant == nil || *gotHant != 3 {
		t.Errorf("zh-Hant preferred: got = %v, want = 3", gotHant)
	}

	// General "zh" should match Chinese subtitles
	gotGeneral := DefaultSubtitleIndex(streams, []string{"zh"}, SubtitlesAlways, "jpn")
	if gotGeneral == nil || (*gotGeneral != 2 && *gotGeneral != 3) {
		t.Errorf("zh general preferred: got = %v, want = 2 or 3", gotGeneral)
	}
}
