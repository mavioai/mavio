package subtitles

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/apps/server/internal/speech"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// fakeRecognizer detects lang and recognizes a segment, recording what
// it is asked.
type fakeRecognizer struct {
	lang string

	mu         sync.Mutex
	detections []string
	recognized []string
}

func (f *fakeRecognizer) ListRecognizers(context.Context, *pluginv1.ListRecognizersRequest) (*pluginv1.ListRecognizersResponse, error) {
	return &pluginv1.ListRecognizersResponse{}, nil
}

func (f *fakeRecognizer) RecognizeSpeech(_ context.Context, req *pluginv1.RecognizeSpeechRequest) (*pluginv1.RecognizeSpeechResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recognized = append(f.recognized, strings.Join([]string{
		req.GetRecognizer(), req.GetAudio().GetUrl(), req.GetLanguage(), map[bool]string{true: "translate"}[req.GetTranslate()], req.GetName(),
	}, " "))
	return pluginv1.RecognizeSpeechResponse_builder{Segments: []*pluginv1.SpeechSegment{
		pluginv1.SpeechSegment_builder{Start: durationpb.New(time.Second), End: durationpb.New(2 * time.Second), Text: proto.String(" Hi ")}.Build(),
	}}.Build(), nil
}

func (f *fakeRecognizer) DetectSpeechLanguage(_ context.Context, req *pluginv1.DetectSpeechLanguageRequest) (*pluginv1.DetectSpeechLanguageResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.detections = append(f.detections, req.GetAudio().GetUrl()+" "+req.GetAudio().GetLimit().AsDuration().String())
	return pluginv1.DetectSpeechLanguageResponse_builder{Language: proto.String(f.lang)}.Build(), nil
}

// whisperInfo describes a recognizer like whisper-asr's.
func whisperInfo(subtitles bool) *pluginv1.RecognizerInfo {
	return pluginv1.RecognizerInfo_builder{
		Name: proto.String("whisper-asr"), DisplayName: proto.String("Whisper"), Enabled: proto.Bool(true),
		Languages: []string{"en", "fr", "de", "jv", "gsw"}, Timestamps: proto.Bool(true), DetectsLanguage: proto.Bool(true),
		Translates: proto.Bool(true), Subtitles: proto.Bool(subtitles),
	}.Build()
}

func newTranscriber(f *fakeRecognizer, infos ...*pluginv1.RecognizerInfo) *transcriber {
	r := &Runtime{Recognizers: func(context.Context) []speech.Recognizer {
		var out []speech.Recognizer
		for _, info := range infos {
			p := speech.RecognizerPlugin{ID: "dev.mavio.voice", Client: f}
			out = append(out, speech.Recognizer{ID: p.ID + "/" + info.GetName(), Plugin: p, Info: info})
		}
		return out
	}}
	return &transcriber{r}
}

func spokenStream(index int32, lang string) *pluginv1.SubtitleAudioStream {
	return pluginv1.SubtitleAudioStream_builder{Index: proto.Int32(index), Language: proto.String(lang)}.Build()
}

func wantedLanguages(tags ...string) []*pluginv1.SubtitleLanguage {
	var out []*pluginv1.SubtitleLanguage
	for _, t := range tags {
		out = append(out, pluginv1.SubtitleLanguage_builder{Language: proto.String(t)}.Build())
	}
	return out
}

func TestTranscriptionSearch(t *testing.T) {
	cases := []struct {
		name    string
		streams []*pluginv1.SubtitleAudioStream
		detect  string
		langs   []string
		// want are the release names found.
		want    []string
		detects int
	}{
		{
			name:    "tagged",
			streams: []*pluginv1.SubtitleAudioStream{spokenStream(0, "fre")},
			langs:   []string{"en", "fr", "de"},
			want:    []string{"translate French audio -> English SRT", "transcribe French audio -> French SRT"},
		},
		{
			name:    "untagged",
			streams: []*pluginv1.SubtitleAudioStream{spokenStream(0, "")},
			detect:  "de",
			langs:   []string{"en", "de"},
			want:    []string{"translate German audio -> English SRT", "transcribe German audio -> German SRT"},
			detects: 1,
		},
		{
			name:    "ambiguous",
			streams: []*pluginv1.SubtitleAudioStream{spokenStream(0, "und"), spokenStream(1, "eng")},
			detect:  "jv",
			langs:   []string{"en", "fr"},
			want:    []string{"translate Javanese audio -> English SRT"},
			detects: 1,
		},
		{
			name:    "undetected",
			streams: []*pluginv1.SubtitleAudioStream{spokenStream(0, "mul")},
			langs:   []string{"en"},
			detects: 1,
		},
		{
			name:    "swiss german",
			streams: []*pluginv1.SubtitleAudioStream{spokenStream(0, "gsw")},
			langs:   []string{"de"},
			want:    []string{"transcribe German audio -> German SRT"},
		},
		{
			name:    "unrecognized",
			streams: []*pluginv1.SubtitleAudioStream{spokenStream(0, "kor")},
			langs:   []string{"ko", "en"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeRecognizer{lang: c.detect}
			tr := newTranscriber(f, whisperInfo(true), pluginv1.RecognizerInfo_builder{Name: proto.String("off"), Timestamps: proto.Bool(true)}.Build())
			req := pluginv1.SearchSubtitlesRequest_builder{
				Video: pluginv1.SubtitleVideo_builder{
					Kind: pluginv1.MediaKind_MEDIA_KIND_EPISODE.Enum(), AudioUrl: proto.String("http://mavio.host/subtitles/audio/tok"), AudioStreams: c.streams,
				}.Build(),
				Languages: wantedLanguages(c.langs...),
			}.Build()
			resp, err := tr.SearchSubtitles(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, s := range resp.GetSubtitles() {
				got = append(got, s.GetReleaseNames()...)
				if s.GetProvider() != "dev.mavio.voice/whisper-asr" || !slices.Equal(s.GetMatches(), []string{"series", "season", "episode"}) || s.GetFormat() != "srt" {
					t.Errorf("subtitle = %v, want whisper-asr's with an episode's matches", s)
				}
			}
			if !slices.Equal(got, c.want) || len(f.detections) != c.detects {
				t.Errorf("found %q with %d detections, want %q with %d", got, len(f.detections), c.want, c.detects)
			}
			if c.detects > 0 && f.detections[0] != "http://mavio.host/subtitles/audio/tok?seconds=30&stream=0 30s" {
				t.Errorf("detected from %q, want the first 30 seconds of the first stream", f.detections[0])
			}
			// Detections are remembered.
			if _, err := tr.SearchSubtitles(t.Context(), req); err != nil || len(f.detections) != c.detects {
				t.Errorf("searched again: %d detections, %v; want %d", len(f.detections), err, c.detects)
			}
		})
	}
}

func TestTranscriptionDownload(t *testing.T) {
	f := &fakeRecognizer{}
	tr := newTranscriber(f, whisperInfo(true))
	v := pluginv1.SubtitleVideo_builder{
		FileName: proto.String("Amélie (2001).mkv"), AudioUrl: proto.String("http://mavio.host/subtitles/audio/tok"),
		AudioStreams: []*pluginv1.SubtitleAudioStream{spokenStream(0, "eng"), spokenStream(1, "fre")},
	}.Build()
	found, err := tr.SearchSubtitles(t.Context(), pluginv1.SearchSubtitlesRequest_builder{Video: v, Languages: wantedLanguages("fr", "en")}.Build())
	if err != nil || len(found.GetSubtitles()) != 2 {
		t.Fatalf("SearchSubtitles = %v, %v; want French and English", found, err)
	}
	for _, s := range found.GetSubtitles() {
		resp, err := tr.DownloadSubtitle(t.Context(), pluginv1.DownloadSubtitleRequest_builder{Provider: proto.String(s.GetProvider()), Id: proto.String(s.GetId()), Video: v}.Build())
		if err != nil {
			t.Fatal(err)
		}
		if resp.GetFormat() != "srt" || string(resp.GetData()) != "1\n00:00:01,000 --> 00:00:02,000\nHi\n\n" {
			t.Errorf("DownloadSubtitle = %q as %s, want the segment as SubRip", resp.GetData(), resp.GetFormat())
		}
	}
	want := []string{
		"whisper-asr http://mavio.host/subtitles/audio/tok?stream=1 fr  Amélie (2001).mkv",
		"whisper-asr http://mavio.host/subtitles/audio/tok?stream=0 en  Amélie (2001).mkv",
	}
	if !slices.Equal(f.recognized, want) {
		t.Errorf("recognized = %q, want = %q", f.recognized, want)
	}

	// Without audio, or recognizers allowed to, nothing is found.
	v.SetAudioUrl("")
	if found, err := tr.SearchSubtitles(t.Context(), pluginv1.SearchSubtitlesRequest_builder{Video: v, Languages: wantedLanguages("fr")}.Build()); err != nil || len(found.GetSubtitles()) != 0 {
		t.Errorf("SearchSubtitles without audio = %v, %v; want nothing", found, err)
	}
	off := newTranscriber(f, whisperInfo(false))
	if list, err := off.ListSubtitleProviders(t.Context(), &pluginv1.ListSubtitleProvidersRequest{}); err != nil || len(list.GetProviders()) != 0 {
		t.Errorf("ListSubtitleProviders without subtitles = %v, %v; want none", list, err)
	}
}
