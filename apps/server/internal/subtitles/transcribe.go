package subtitles

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/apps/server/internal/speech"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/subtitle"
)

// TranscriptionID is the ID of the subtitle source transcribing videos'
// audio with the speech recognizers the administrator lets transcribe
// subtitles, as Bazarr's Whisper provider: its subtitles are those of
// providers "transcription/<recognizer>".
const TranscriptionID = "transcription"

const (
	// detectLength is how much audio languages are detected from: Whisper
	// detects them from the first 30 seconds.
	detectLength = 30 * time.Second
	// detectedSize bounds the languages detected kept, as Bazarr's
	// lru_cache(2048).
	detectedSize = 2048
)

// ambiguousCodes are the language codes of audio streams that do not say
// their language, whose language is detected.
var ambiguousCodes = []string{"alg", "art", "ath", "aus", "mis", "mul", "sgn", "und", "zxx"}

// writtenAs are languages transcribed as another they are written in:
// Swiss German as German.
var writtenAs = map[string]string{"gsw": "de"}

// transcriber is the transcription source, as a subtitle provider plugin.
type transcriber struct{ r *Runtime }

// transcription is a subtitle a search found, as its ID holds it.
type transcription struct {
	// Task is "transcribe" or "translate".
	Task string `json:"task"`
	// Audio is the audio's language, BCP 47.
	Audio string `json:"audio"`
	// Stream is the audio stream transcribed.
	Stream int `json:"stream"`
}

// recognizers returns the recognizers that transcribe subtitles.
func (tr *transcriber) recognizers(ctx context.Context) []speech.Recognizer {
	if tr.r.Recognizers == nil {
		return nil
	}
	var out []speech.Recognizer
	for _, rec := range tr.r.Recognizers(ctx) {
		if rec.Info.GetSubtitles() && rec.Info.GetTimestamps() {
			out = append(out, rec)
		}
	}
	return out
}

// ListSubtitleProviders implements SubtitleProviderServiceClient: a
// provider per recognizer.
func (tr *transcriber) ListSubtitleProviders(ctx context.Context, _ *pluginv1.ListSubtitleProvidersRequest) (*pluginv1.ListSubtitleProvidersResponse, error) {
	var out []*pluginv1.SubtitleProviderInfo
	for _, rec := range tr.recognizers(ctx) {
		out = append(out, pluginv1.SubtitleProviderInfo_builder{
			Name: proto.String(rec.ID), DisplayName: proto.String(rec.Info.GetDisplayName()), Enabled: proto.Bool(true),
			Languages: rec.Info.GetLanguages(), Movies: proto.Bool(true), Episodes: proto.Bool(true),
		}.Build())
	}
	return pluginv1.ListSubtitleProvidersResponse_builder{Providers: out}.Build(), nil
}

// SearchSubtitles implements SubtitleProviderServiceClient: a subtitle
// per recognizer and language the audio can be transcribed or translated
// to.
func (tr *transcriber) SearchSubtitles(ctx context.Context, req *pluginv1.SearchSubtitlesRequest) (*pluginv1.SearchSubtitlesResponse, error) {
	v := req.GetVideo()
	resp := &pluginv1.SearchSubtitlesResponse{}
	if v.GetAudioUrl() == "" {
		return resp, nil
	}
	matches := []string{"title"}
	if v.GetKind() == pluginv1.MediaKind_MEDIA_KIND_EPISODE {
		matches = []string{"series", "season", "episode"}
	}
	for _, rec := range tr.recognizers(ctx) {
		for _, l := range req.GetLanguages() {
			t, err := tr.query(ctx, rec, v, l.GetLanguage())
			if err != nil {
				tr.r.logger().WarnContext(ctx, "transcription search", "recognizer", rec.ID, "err", err)
				resp.SetFailures(append(resp.GetFailures(), pluginv1.SubtitleProviderFailure_builder{
					Provider: proto.String(rec.ID), Message: proto.String(err.Error()),
				}.Build()))
				break
			}
			if t == nil {
				continue
			}
			id, err := json.Marshal(t)
			if err != nil {
				return nil, err
			}
			resp.SetSubtitles(append(resp.GetSubtitles(), pluginv1.RemoteSubtitle_builder{
				Provider: proto.String(rec.ID), Id: proto.String(string(id)), Language: l, Format: proto.String(subtitle.SRT),
				ReleaseNames: []string{fmt.Sprintf("%s %s audio -> %s SRT", t.Task, languageName(t.Audio, false), languageName(l.GetLanguage(), false))},
				Matches:      matches,
			}.Build()))
		}
	}
	return resp, nil
}

// spoken returns the BCP 47 tag of an audio stream's language as it is
// transcribed.
func spoken(code string) string {
	t := Tag(code)
	if w, ok := writtenAs[t]; ok {
		return w
	}
	return t
}

// query decides how a recognizer transcribes a video into a language, as
// Bazarr's Whisper provider; nil when it does not.
func (tr *transcriber) query(ctx context.Context, rec speech.Recognizer, v *pluginv1.SubtitleVideo, target string) (*transcription, error) {
	info := rec.Info
	if !speech.Speaks(info.GetLanguages(), target) {
		return nil, nil
	}
	t := &transcription{Task: "transcribe"}
	var tagged []*pluginv1.SubtitleAudioStream
	for _, s := range v.GetAudioStreams() {
		if s.GetLanguage() != "" {
			tagged = append(tagged, s)
		}
	}
	detect := len(tagged) == 0
	if !detect {
		i := slices.IndexFunc(tagged, func(s *pluginv1.SubtitleAudioStream) bool {
			return speech.Base(spoken(s.GetLanguage())) == speech.Base(target)
		})
		if i >= 0 {
			t.Audio = spoken(tagged[i].GetLanguage())
			if len(tagged) > 1 {
				t.Stream = int(tagged[i].GetIndex())
			}
		} else {
			t.Task, t.Audio = "translate", spoken(tagged[0].GetLanguage())
		}
		detect = slices.ContainsFunc(tagged, func(s *pluginv1.SubtitleAudioStream) bool {
			return slices.Contains(ambiguousCodes, strings.ToLower(s.GetLanguage()))
		})
	}
	if detect {
		if !info.GetDetectsLanguage() {
			return nil, nil
		}
		detected, err := tr.r.detect(ctx, rec, v.GetAudioUrl())
		if err != nil {
			return nil, err
		}
		if detected == "" {
			tr.r.logger().InfoContext(ctx, "language detection failed", "recognizer", rec.ID, "file", v.GetFileName())
			return nil, nil
		}
		t.Audio, t.Task, t.Stream = detected, "transcribe", 0
		if speech.Base(detected) != speech.Base(target) {
			t.Task = "translate"
		}
	}
	if t.Audio == "" || !speech.Speaks(info.GetLanguages(), t.Audio) {
		return nil, nil
	}
	if t.Task == "translate" && (speech.Base(target) != "en" || !info.GetTranslates()) {
		return nil, nil
	}
	return t, nil
}

// detect returns the language of a video's first audio stream as a
// recognizer detects it from its first 30 seconds; "" when it cannot be.
// Detections are kept by recognizer and audio.
func (r *Runtime) detect(ctx context.Context, rec speech.Recognizer, audioURL string) (string, error) {
	key := rec.ID + "\x00" + audioURL
	r.detectedMu.Lock()
	lang, ok := r.detected[key]
	r.detectedMu.Unlock()
	if ok {
		return lang, nil
	}
	lang, err := rec.DetectLanguage(ctx, pluginv1.SpeechAudio_builder{
		Url: proto.String(streamURL(audioURL, 0, detectLength)), Limit: durationpb.New(detectLength),
	}.Build())
	if err != nil {
		return "", err
	}
	r.detectedMu.Lock()
	defer r.detectedMu.Unlock()
	if r.detected == nil || len(r.detected) >= detectedSize {
		r.detected = map[string]string{}
	}
	r.detected[key] = lang
	return lang, nil
}

// streamURL returns the URL of an audio stream of the audio route, or of
// its first seconds when d is positive.
func streamURL(audioURL string, stream int, d time.Duration) string {
	q := url.Values{"stream": {strconv.Itoa(stream)}}
	if d > 0 {
		q.Set("seconds", strconv.Itoa(int(d/time.Second)))
	}
	return audioURL + "?" + q.Encode()
}

// DownloadSubtitle implements SubtitleProviderServiceClient: the
// recognizer transcribes the audio stream, as SubRip.
func (tr *transcriber) DownloadSubtitle(ctx context.Context, req *pluginv1.DownloadSubtitleRequest) (*pluginv1.DownloadSubtitleResponse, error) {
	var t transcription
	if err := json.Unmarshal([]byte(req.GetId()), &t); err != nil {
		return nil, fmt.Errorf("invalid subtitle ID %q: %w", req.GetId(), err)
	}
	v := req.GetVideo()
	if v.GetAudioUrl() == "" {
		return nil, fmt.Errorf("%w: the video's audio is not served", ErrUnavailable)
	}
	i := slices.IndexFunc(tr.recognizers(ctx), func(rec speech.Recognizer) bool { return rec.ID == req.GetProvider() })
	if i < 0 {
		return nil, fmt.Errorf("%w: recognizer %s does not transcribe subtitles", ErrUnavailable, req.GetProvider())
	}
	rec := tr.recognizers(ctx)[i]
	start := tr.r.now()
	tr.r.logger().InfoContext(ctx, "transcription starting", "recognizer", rec.ID, "task", t.Task, "file", v.GetFileName())
	resp, err := rec.Recognize(ctx, pluginv1.RecognizeSpeechRequest_builder{
		Audio:    pluginv1.SpeechAudio_builder{Url: proto.String(streamURL(v.GetAudioUrl(), t.Stream, 0))}.Build(),
		Language: proto.String(t.Audio), Translate: proto.Bool(t.Task == "translate"), Timestamps: proto.Bool(true),
		Name: proto.String(v.GetFileName()),
	}.Build())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rec.ID, err)
	}
	tr.r.logger().InfoContext(ctx, "transcription completed", "recognizer", rec.ID, "took", tr.r.now().Sub(start).Round(time.Second), "file", v.GetFileName())
	var cues []subtitle.Cue
	for _, s := range resp.GetSegments() {
		if text := strings.TrimSpace(s.GetText()); text != "" {
			cues = append(cues, subtitle.Cue{Index: len(cues) + 1, Start: s.GetStart().AsDuration(), End: s.GetEnd().AsDuration(), Text: text})
		}
	}
	if len(cues) == 0 {
		return pluginv1.DownloadSubtitleResponse_builder{Failure: pluginv1.SubtitleProviderFailure_builder{
			Provider: proto.String(rec.ID), Message: proto.String("nothing was said"),
			Kind: pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_BAD_SUBTITLE.Enum(),
		}.Build()}.Build(), nil
	}
	data, err := subtitle.Write(&subtitle.Subtitle{Cues: cues}, subtitle.SRT)
	if err != nil {
		return nil, err
	}
	return pluginv1.DownloadSubtitleResponse_builder{Data: data, Format: proto.String(subtitle.SRT)}.Build(), nil
}
