package decision

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// Transcoder reports what the server's ffmpeg can produce.
type Transcoder interface {
	// CanEncodeAudio reports whether audio can be encoded to codec.
	CanEncodeAudio(codec string) bool
	// CanExtractSubtitles reports whether embedded subtitles in codec can
	// be extracted to a separate file.
	CanExtractSubtitles(codec string) bool
}

// Request asks how to play one of the sources of an item on a client.
type Request struct {
	Sources []*Source
	// SourceID restricts the decision to one source; zero considers all.
	SourceID core.ID
	Client   *ClientCapabilities
	Context  Context

	// MaxBitrate caps the bitrate below the client's limits, in bits per
	// second; 0 uses the client's.
	MaxBitrate int64
	// MaxAudioChannels caps the output channels; 0 means no cap.
	MaxAudioChannels int
	// AudioTranscodingBitrate is the bitrate audio-only transcodes target;
	// 0 uses the client's.
	AudioTranscodingBitrate int64
	// AudioStreamIndex and SubtitleStreamIndex select streams; nil uses the
	// source's defaults. They require SourceID.
	AudioStreamIndex, SubtitleStreamIndex *int

	EnableDirectPlay   bool
	EnableDirectStream bool
	// ForceDirectPlay and ForceDirectStream skip the checks.
	ForceDirectPlay   bool
	ForceDirectStream bool
	// AllowVideoStreamCopy and AllowAudioStreamCopy let transcodes copy
	// streams the client plays.
	AllowVideoStreamCopy bool
	AllowAudioStreamCopy bool

	AlwaysBurnInSubtitleWhenTranscoding bool
}

// maxBitrate returns the bitrate cap for audio or video, 0 for none.
func (r *Request) maxBitrate(audio bool) int64 {
	if r.MaxBitrate > 0 {
		return r.MaxBitrate
	}
	if r.Context == Static {
		if audio && r.Client.MaxStaticMusicBitrate > 0 {
			return r.Client.MaxStaticMusicBitrate
		}
		return r.Client.MaxStaticBitrate
	}
	return r.Client.MaxStreamingBitrate
}

// ErrInvalidRequest is returned for requests that cannot be decided.
var ErrInvalidRequest = errors.New("invalid playback request")

func (r *Request) validate(video bool) error {
	switch {
	case r.Client == nil:
		return fmt.Errorf("%w: no client capabilities", ErrInvalidRequest)
	case video && r.SourceID.IsZero() && (r.AudioStreamIndex != nil || r.SubtitleStreamIndex != nil):
		return fmt.Errorf("%w: selecting a stream requires a source", ErrInvalidRequest)
	}
	return nil
}

func (r *Request) sources() []*Source {
	if r.SourceID.IsZero() {
		return r.Sources
	}
	var out []*Source
	for _, s := range r.Sources {
		if s.ID == r.SourceID {
			out = append(out, s)
		}
	}
	return out
}

// Builder decides how media is played.
type Builder struct {
	Transcoder Transcoder
	Logger     *slog.Logger
}

func (b *Builder) logger() *slog.Logger {
	if b.Logger != nil {
		return b.Logger
	}
	return slog.New(slog.DiscardHandler)
}

// manifestContainers describe streams rather than carry them, so they are
// never direct played: their URIs are relative to their origin.
const manifestContainers Names = "hls,applehttp,dash"

var (
	hlsVideoCodecs    = []string{"h264", "hevc", "vp9", "av1"}
	hlsAudioCodecsTS  = []string{"aac", "ac3", "eac3", "mp3"}
	hlsAudioCodecsMP4 = []string{"aac", "ac3", "eac3", "mp3", "alac", "flac", "opus", "dts", "truehd"}
)

// Video decides how to play a video item; it returns nil when no source
// can be played.
func (b *Builder) Video(r *Request) (*Decision, error) {
	if err := r.validate(true); err != nil {
		return nil, err
	}
	var ds []*Decision
	for _, s := range r.sources() {
		ds = append(ds, b.video(s, r))
	}
	return best(ds, r.maxBitrate(false)), nil
}

// Audio decides how to play an audio item; it returns nil when no source
// can be played.
func (b *Builder) Audio(r *Request) (*Decision, error) {
	if err := r.validate(false); err != nil {
		return nil, err
	}
	var ds []*Decision
	for _, s := range r.sources() {
		if d := b.audio(s, r); d != nil {
			ds = append(ds, d)
		}
	}
	return best(ds, r.maxBitrate(true)), nil
}

// best prefers direct play of local files, then direct play and direct
// stream, then local files, then the bitrate closest to the cap.
func best(ds []*Decision, maxBitrate int64) *Decision {
	key := func(d *Decision) [4]int64 {
		var k [4]int64
		if d.Method != DirectPlay || d.Source.Remote {
			k[0] = 1
		}
		if d.Method != DirectStream && d.Method != DirectPlay {
			k[1] = 1
		}
		if d.Source.Remote {
			k[2] = 1
		}
		if maxBitrate > 0 && d.Source.Bitrate > 0 {
			k[3] = abs(d.Source.Bitrate - maxBitrate)
		}
		return k
	}
	sorted := slices.Clone(ds)
	slices.SortStableFunc(sorted, func(a, b *Decision) int {
		ka, kb := key(a), key(b)
		return slices.Compare(ka[:], kb[:])
	})
	if len(sorted) == 0 {
		return nil
	}
	return sorted[0]
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// singleContainer picks the first of a source's comma-separated container
// names that a direct play profile of kind supports.
func singleContainer(container string, c *ClientCapabilities, kind MediaKind, played *DirectPlayProfile) string {
	if c == nil || container == "" || !strings.Contains(container, ",") {
		return container
	}
	profiles := c.DirectPlay
	if played != nil {
		profiles = []DirectPlayProfile{*played}
	}
	for _, format := range split(container) {
		for i := range profiles {
			if profiles[i].Kind == kind && profiles[i].supportsContainer(format) {
				return format
			}
		}
	}
	return container
}

func (b *Builder) bitrateExceeded(s *Source, maxBitrate int64) bool {
	if s.Remote {
		return false
	}
	limit := maxBitrate
	if limit <= 0 {
		limit = math.MaxInt32
	}
	// An unknown bitrate is assumed to be 40 Mbit/s.
	bitrate := s.Bitrate
	if bitrate <= 0 {
		bitrate = 40_000_000
	}
	if bitrate > limit {
		b.logger().Debug("bitrate exceeds limit", "bitrate", bitrate, "limit", limit)
		return true
	}
	return false
}

func newDecision(s *Source, kind MediaKind, r *Request) *Decision {
	return &Decision{Source: s, Kind: kind, Context: r.Context, Method: Transcode, Protocol: HTTP, SubtitleMethod: SubtitleEncode, SeekInfo: SeekAuto}
}

func (b *Builder) video(s *Source, r *Request) *Decision {
	d := newDecision(s, Video, r)
	d.AlwaysBurnInSubtitleWhenTranscoding = r.AlwaysBurnInSubtitleWhenTranscoding
	d.SubtitleStreamIndex = r.SubtitleStreamIndex
	if d.SubtitleStreamIndex == nil {
		d.SubtitleStreamIndex = defaultSubtitleIndex(s, r.Client.Subtitles)
	}
	var subtitle *core.MediaStream
	if d.SubtitleStreamIndex != nil {
		subtitle = s.stream(core.StreamSubtitle, *d.SubtitleStreamIndex)
	}
	audioIndex := r.AudioStreamIndex
	if audioIndex == nil {
		audioIndex = s.DefaultAudioIndex
	}
	audio := s.defaultAudioStream(audioIndex)
	if audio != nil {
		d.AudioStreamIndex = ptr(audio.Index)
	}
	candidates := audioCandidates(s, r, audio)

	video := s.videoStream()
	exceeded := b.bitrateExceeded(s, r.maxBitrate(false))
	directPlayEligible := r.EnableDirectPlay && (r.ForceDirectPlay || !exceeded)
	directStreamEligible := r.EnableDirectStream && (r.ForceDirectStream || !exceeded)
	if s.Disc || manifestContainers.Contains(s.Container) {
		directPlayEligible = false
	}
	var reasons Reasons
	if exceeded {
		reasons = ContainerBitrateExceedsLimit
	}

	var played *DirectPlayProfile
	if directPlayEligible || directStreamEligible {
		m := b.videoDirectPlay(r, s, video, audio, candidates, subtitle, directPlayEligible, directStreamEligible)
		reasons |= m.reasons
		if m.method != "" {
			played = m.profile
			d.Method = m.method
			d.Container = singleContainer(s.Container, r.Client, Video, played)
			d.VideoCodecs = nil
			if video != nil {
				d.VideoCodecs = []string{video.Codec}
			}
			var playedContainer string
			if played != nil {
				playedContainer = played.Container.String()
			}
			switch m.method {
			case DirectPlay:
				d.Protocol = HTTP
				idx := m.audioIndex
				if idx == nil && audio != nil {
					idx = ptr(audio.Index)
				}
				if idx != nil {
					d.AudioStreamIndex = idx
					d.AudioCodecs = nil
					if a := s.stream(core.StreamAudio, *idx); a != nil {
						d.AudioCodecs = []string{a.Codec}
					}
				}
			case DirectStream:
				d.AudioStreamIndex = streamIndex(audio)
				d.Protocol = HTTP
				d.AudioCodecs = played.audioCodecs()
				b.streamVideo(d, r, s, video, audio, candidates, playedContainer, played.videoCodec(), played.audioCodec())
			}
			if subtitle != nil {
				p := subtitleProfile(s, subtitle, r.Client.Subtitles, m.method, b.Transcoder, playedContainer, "")
				d.SubtitleMethod, d.SubtitleFormat = p.Method, p.Format
			}
		}
		b.logger().Debug("direct play result", "client", r.Client.Name, "path", s.Path, "method", m.method, "reasons", m.reasons)
	}
	d.Reasons = reasons

	if d.Method != DirectStream && d.Method != DirectPlay {
		if tp, method := b.videoTranscodeProfile(s, r, video, audio, d); tp != nil && method != "" {
			b.applyTranscodingProfile(d, tp)
			b.streamVideo(d, r, s, video, audio, candidates, tp.Container, tp.VideoCodec, tp.AudioCodec)
			d.Method = Transcode
			if subtitle != nil {
				p := subtitleProfile(s, subtitle, r.Client.Subtitles, Transcode, b.Transcoder, tp.Container, tp.Protocol)
				d.SubtitleMethod, d.SubtitleFormat = p.Method, p.Format
				d.SubtitleCodecs = []string{p.Format}
			}
			if d.Reasons&(videoReasons|ContainerBitrateExceedsLimit) != 0 {
				b.applyConditions(d, tp.Conditions, "", true, true)
			}
		}
	}
	b.logger().Debug("video decision", "client", r.Client.Name, "path", s.Path, "method", d.Method, "reasons", d.Reasons)
	return d
}

func ptr[T any](v T) *T { return &v }

// String returns the list as declared.
func (n Names) String() string { return string(n) }

func (p *DirectPlayProfile) videoCodec() Names {
	if p == nil {
		return ""
	}
	return p.VideoCodec
}

func (p *DirectPlayProfile) audioCodec() Names {
	if p == nil {
		return ""
	}
	return p.AudioCodec
}

func (p *DirectPlayProfile) audioCodecs() []string { return p.audioCodec().rawList() }

// rawList splits the list as declared, keeping a "-" prefix.
func (n Names) rawList() []string { return split(string(n)) }

// audioCandidates returns the audio streams a decision may switch to when
// the selected one cannot be played: none when the client or the user chose
// the stream, streams of the selected language or with the default flag
// when those decided the choice, and otherwise all.
func audioCandidates(s *Source, r *Request, audio *core.MediaStream) []*core.MediaStream {
	var candidates []*core.MediaStream
	if audio != nil {
		candidates = []*core.MediaStream{audio}
	}
	if s.AudioIndexSource&AudioIndexUser != 0 || (r.AudioStreamIndex != nil && *r.AudioStreamIndex >= 0) {
		return candidates
	}
	audios := func(keep func(*core.MediaStream) bool) []*core.MediaStream {
		var out []*core.MediaStream
		for i := range s.Streams {
			if st := &s.Streams[i]; st.Kind == core.StreamAudio && keep(st) {
				out = append(out, st)
			}
		}
		return out
	}
	if s.AudioIndexSource == 0 && audio != nil {
		candidates = audios(func(*core.MediaStream) bool { return true })
		if audio.Default {
			candidates = audios(func(st *core.MediaStream) bool { return st.Default })
		}
	}
	var lang string
	if audio != nil {
		lang = audio.Language
	}
	switch {
	case s.AudioIndexSource&AudioIndexLanguage != 0:
		candidates = audios(func(st *core.MediaStream) bool { return st.Language == lang })
		if s.AudioIndexSource&AudioIndexDefault != 0 {
			defaults := audios(func(st *core.MediaStream) bool { return st.Language == lang && st.Default })
			if len(defaults) == 0 {
				defaults = audios(func(st *core.MediaStream) bool { return st.Default })
			}
			candidates = defaults
		}
	case s.AudioIndexSource&AudioIndexDefault != 0:
		candidates = audios(func(st *core.MediaStream) bool { return st.Default })
	}
	return candidates
}

// defaultSubtitleIndex picks, among the best scored subtitle streams, the
// first a client takes as an external file without conversion, else the
// source's default.
func defaultSubtitleIndex(s *Source, profiles []SubtitleProfile) *int {
	highest := -1
	for i := range s.Streams {
		if st := &s.Streams[i]; st.Kind == core.StreamSubtitle {
			if score, ok := s.SubtitleScores[st.Index]; ok && score > highest {
				highest = score
			}
		}
	}
	var top []*core.MediaStream
	for i := range s.Streams {
		if st := &s.Streams[i]; st.Kind == core.StreamSubtitle {
			if score, ok := s.SubtitleScores[st.Index]; ok && score == highest {
				top = append(top, st)
			}
		}
	}
	if len(top) > 1 {
		for _, st := range top {
			for i := range profiles {
				p := &profiles[i]
				if p.Method == SubtitleExternal && (isVobSubMKSProfile(p, st) ||
					(!isVobSubMKSDelivery(p) && strings.EqualFold(p.Format, st.Codec))) {
					return ptr(st.Index)
				}
			}
		}
	}
	return s.DefaultSubtitleIndex
}

// directPlayMatch is the outcome of matching direct play profiles.
type directPlayMatch struct {
	profile    *DirectPlayProfile
	method     Method
	audioIndex *int
	reasons    Reasons
}

func (b *Builder) videoDirectPlay(r *Request, s *Source, video, audio *core.MediaStream, candidates []*core.MediaStream, subtitle *core.MediaStream, directPlayEligible, directStreamEligible bool) directPlayMatch {
	if r.ForceDirectPlay {
		return directPlayMatch{method: DirectPlay, audioIndex: streamIndex(audio)}
	}
	if r.ForceDirectStream {
		return directPlayMatch{method: DirectStream, audioIndex: streamIndex(audio)}
	}
	c := r.Client
	container := s.Container
	containerReasons := b.containerCompatibility(r, s, container, video)
	var videoCodecReasons Reasons
	if video != nil {
		videoCodecReasons = b.videoCodecCompatibility(r, s, container, video)
	}
	audioReasonsOf := make(map[*core.MediaStream]Reasons, len(candidates))
	for _, a := range candidates {
		audioReasonsOf[a] = b.audioCompatibilityDirect(r, s, container, a, true, s.isSecondaryAudio(a).v)
	}
	var subtitleReasons Reasons
	if subtitle != nil {
		p := subtitleProfile(s, subtitle, c.Subtitles, DirectPlay, b.Transcoder, container, "")
		if p.Method != SubtitleDrop && p.Method != SubtitleExternal && p.Method != SubtitleEmbed {
			b.logger().Debug("not eligible for direct play due to unsupported subtitles")
			subtitleReasons |= SubtitleCodecNotSupported
		}
	}

	containerSupported := false
	rankings := []Reasons{VideoCodecNotSupported, videoCodecReasons, AudioCodecNotSupported, audioCodecReasons, containerReasons}
	type analysis struct {
		match directPlayMatch
		order int
		rank  int
	}
	var analyses []analysis
	for i := range c.DirectPlay {
		p := &c.DirectPlay[i]
		if p.Kind != Video {
			continue
		}
		var profileReasons, audioCodecProfileReasons Reasons
		if !p.supportsContainer(container) {
			profileReasons |= ContainerNotSupported
		} else {
			containerSupported = true
		}
		var videoCodec string
		if video != nil {
			videoCodec = video.Codec
		}
		if !p.supportsVideoCodec(videoCodec) {
			profileReasons |= VideoCodecNotSupported
		}
		var selected *core.MediaStream
		if len(candidates) != 0 {
			for _, a := range candidates {
				if p.supportsAudioCodec(a.Codec) {
					selected = a
					break
				}
			}
			if selected == nil {
				profileReasons |= AudioCodecNotSupported
			} else {
				audioCodecProfileReasons = audioReasonsOf[selected]
			}
		}
		failures := profileReasons | containerReasons | subtitleReasons
		if failures&VideoCodecNotSupported == 0 {
			failures |= videoCodecReasons
		}
		if failures&AudioCodecNotSupported == 0 {
			failures |= audioCodecProfileReasons
		}
		directStreamFailures := failures &^ directStreamReasons
		var method Method
		switch {
		case failures == 0 && directPlayEligible && !s.NoDirectPlay:
			method = DirectPlay
		case directStreamFailures == 0 && directStreamEligible && !s.NoDirectStream:
			method = DirectStream
		}
		analyses = append(analyses, analysis{
			match: directPlayMatch{profile: p, method: method, audioIndex: streamIndex(selected), reasons: failures},
			order: len(analyses),
			rank:  rankOf(failures, rankings),
		})
	}
	// Prefer the best method, then the profile whose most significant
	// failure is least important, then declaration order.
	methodRank := func(m Method) int {
		if m == "" {
			return -1
		}
		return m.rank()
	}
	slices.SortStableFunc(analyses, func(a, b analysis) int {
		return cmp.Or(
			cmp.Compare(methodRank(b.match.method), methodRank(a.match.method)),
			cmp.Compare(b.rank, a.rank),
			cmp.Compare(a.order, b.order),
		)
	})
	for _, a := range analyses {
		if a.match.method != "" {
			return a.match
		}
	}
	var failures Reasons
	for _, a := range analyses {
		if !containerSupported || a.match.reasons&ContainerNotSupported == 0 {
			failures = a.match.reasons
			break
		}
	}
	if failures == 0 {
		failures = DirectPlayError
	}
	return directPlayMatch{reasons: failures}
}

func streamIndex(st *core.MediaStream) *int {
	if st == nil {
		return nil
	}
	return ptr(st.Index)
}

// rankOf returns the 1-based position of the first ranking group that has
// any of the reasons, or one past the last.
func rankOf(reasons Reasons, rankings []Reasons) int {
	for i, r := range rankings {
		if reasons&r != 0 {
			return i + 1
		}
	}
	return len(rankings) + 1
}

func (b *Builder) containerCompatibility(r *Request, s *Source, container string, video *core.MediaStream) Reasons {
	facts := newVideoFacts(s, video)
	var failed []Condition
	for i := range r.Client.Containers {
		p := &r.Client.Containers[i]
		if p.Kind == Video && p.containsContainer(container) {
			failed = append(failed, facts.failing(p.Conditions)...)
		}
	}
	b.logFailures(r, s, "container profile", failed)
	return reasonsOf(failed)
}

func (b *Builder) videoCodecCompatibility(r *Request, s *Source, container string, video *core.MediaStream) Reasons {
	facts := newVideoFacts(s, video)
	var failed []Condition
	for i := range r.Client.Codecs {
		p := &r.Client.Codecs[i]
		if p.Kind == CodecVideo && p.containsCodec(video.Codec, container, false) && len(facts.failing(p.ApplyConditions)) == 0 {
			failed = append(failed, facts.failing(p.Conditions)...)
		}
	}
	b.logFailures(r, s, "video codec profile", failed)
	return reasonsOf(failed)
}

// audioCompatibility returns why an audio stream, encoded as codec when
// given, fails the client's codec profiles.
func (b *Builder) audioCompatibility(r *Request, s *Source, container string, a *core.MediaStream, codec string, inVideo, secondary bool) Reasons {
	if codec == "" {
		codec = a.Codec
	}
	facts := newAudioFacts(a, some(secondary))
	kind := CodecAudio
	if inVideo {
		kind = CodecVideoAudio
	}
	var failed []Condition
	for i := range r.Client.Codecs {
		p := &r.Client.Codecs[i]
		if p.Kind != kind || !p.containsCodec(codec, container, false) || !allSatisfied(p.ApplyConditions, &facts, inVideo) {
			continue
		}
		for _, cond := range p.Conditions {
			if !facts.satisfies(cond, inVideo) {
				failed = append(failed, cond)
			}
		}
	}
	b.logFailures(r, s, "audio codec profile", failed)
	return reasonsOf(failed)
}

func allSatisfied(conds []Condition, facts *audioFacts, inVideo bool) bool {
	for _, c := range conds {
		if !facts.satisfies(c, inVideo) {
			return false
		}
	}
	return true
}

func (b *Builder) audioCompatibilityDirect(r *Request, s *Source, container string, a *core.MediaStream, inVideo, secondary bool) Reasons {
	reasons := b.audioCompatibility(r, s, container, a, "", inVideo, secondary)
	if isExternal(a) {
		reasons |= AudioIsExternal
	}
	return reasons
}

func (b *Builder) logFailures(r *Request, s *Source, kind string, failed []Condition) {
	for _, c := range failed {
		b.logger().Debug("condition failed", "client", r.Client.Name, "profile", kind,
			"property", c.Property, "op", c.Op, "value", c.Value, "optional", c.Optional, "path", s.Path)
	}
}

// videoTranscodeProfile picks the transcoding profile that copies the most:
// one that can copy the video, then one that copies or satisfiably
// transcodes the audio.
func (b *Builder) videoTranscodeProfile(s *Source, r *Request, video, audio *core.MediaStream, d *Decision) (*TranscodingProfile, Method) {
	if s.NoTranscoding && s.NoDirectStream {
		return nil, ""
	}
	var videoCodec, audioCodec string
	if video != nil {
		videoCodec = video.Codec
	}
	if audio != nil {
		audioCodec = audio.Codec
	}
	type analysis struct {
		profile      *TranscodingProfile
		method       Method
		video, audio int
	}
	var analyses []analysis
	for i := range r.Client.Transcoding {
		tp := &r.Client.Transcoding[i]
		if tp.Kind != d.Kind || tp.Context != r.Context {
			continue
		}
		if s.MostCompatible && !strings.EqualFold(tp.Container, "ts") {
			continue
		}
		a := analysis{profile: tp, method: Transcode, video: 3, audio: 3}
		if video != nil && r.AllowVideoStreamCopy && tp.VideoCodec.Contains(videoCodec) {
			if b.videoCodecCompatibility(r, s, tp.Container, video) == 0 {
				a.video = 1
			} else {
				a.video = 2
			}
		}
		if audio != nil && r.AllowAudioStreamCopy {
			// Prefer copying the audio, then a codec that satisfies the
			// client's conditions: 6-channel FLAC goes to 6-channel AAC
			// rather than being downmixed to the 2 channels FLAC allows.
			for _, codec := range tp.AudioCodec.rawList() {
				rank := 3
				if b.audioCompatibility(r, s, tp.Container, audio, codec, true, false) == 0 {
					rank = 2
					if strings.EqualFold(codec, audioCodec) {
						rank = 1
					}
				}
				a.audio = min(a.audio, rank)
				if a.audio == 1 {
					break
				}
			}
		}
		if a.video == 1 {
			a.method = DirectStream
		}
		analyses = append(analyses, a)
	}
	slices.SortStableFunc(analyses, func(x, y analysis) int {
		return cmp.Or(cmp.Compare(x.video, y.video), cmp.Compare(x.audio, y.audio))
	})
	if len(analyses) == 0 {
		return nil, ""
	}
	return analyses[0].profile, analyses[0].method
}

func (b *Builder) applyTranscodingProfile(d *Decision, tp *TranscodingProfile) {
	if d.Method == Transcode {
		d.Container = tp.Container
		d.Protocol = tp.Protocol
	}
	d.SeekInfo = tp.SeekInfo
	if d.SeekInfo == "" {
		d.SeekInfo = SeekAuto
	}
	if tp.MaxAudioChannels > 0 {
		d.TranscodingMaxAudioChannels = tp.MaxAudioChannels
	}
	d.EstimateContentLength = tp.EstimateContentLength
	d.CopyTimestamps = tp.CopyTimestamps
	d.EnableSubtitlesInManifest = tp.EnableSubtitlesInManifest
	d.EnableMpegtsM2TsMode = tp.EnableMpegtsM2TsMode
	d.DisableAudioVBR = tp.DisableAudioVBR
	if tp.MinSegments > 0 {
		d.MinSegments = tp.MinSegments
	}
	if tp.SegmentLength > 0 {
		d.SegmentLength = tp.SegmentLength
	}
}

// streamVideo fills in the output codecs and limits of a remux or
// transcode to container with the given codec lists.
func (b *Builder) streamVideo(d *Decision, r *Request, s *Source, video, audio *core.MediaStream, candidates []*core.MediaStream, container string, videoCodec, audioCodec Names) {
	videoCodecs := videoCodec.rawList()
	if len(videoCodecs) == 0 && video != nil {
		videoCodecs = []string{video.Codec}
	}
	if d.Protocol == HLS {
		videoCodecs = slices.DeleteFunc(videoCodecs, func(c string) bool { return !slices.Contains(hlsVideoCodecs, c) })
	}
	d.VideoCodecs = videoCodecs
	if video != nil && !containsIn(videoCodecs, video.Codec) {
		d.Reasons |= VideoCodecNotSupported
	}

	// Copy the source's video properties as a starting point.
	d.MaxFramerate = 0
	var qualifier string
	if video != nil {
		if fr, ok := referenceFrameRate(video); ok {
			d.MaxFramerate = fr
		}
		qualifier = video.Codec
		if video.Level != 0 {
			d.setOption(qualifier, "level", strconv.Itoa(video.Level))
		}
		if video.BitDepth != 0 {
			d.setOption(qualifier, "videobitdepth", strconv.Itoa(video.BitDepth))
		}
		if video.Profile != "" {
			d.setOption(qualifier, "profile", strings.ToLower(video.Profile))
		}
	}

	audioCodecs := audioCodec.rawList()
	if len(audioCodecs) == 0 && audio != nil {
		audioCodecs = []string{audio.Codec}
	}
	if d.Protocol == HLS {
		allowed := hlsAudioCodecsTS
		if strings.EqualFold(d.Container, "mp4") {
			allowed = hlsAudioCodecsMP4
		}
		audioCodecs = slices.DeleteFunc(audioCodecs, func(c string) bool { return !slices.Contains(allowed, c) })
	}
	var supported *core.MediaStream
	for _, a := range candidates {
		if containsIn(audioCodecs, a.Codec) {
			supported = a
			break
		}
	}
	channelLimit := d.TranscodingMaxAudioChannels
	if channelLimit == 0 {
		channelLimit = math.MaxInt
	}
	channelsExceeded := supported != nil && supported.Channels != 0 && supported.Channels > channelLimit
	var directAudioFailures Reasons
	if supported != nil {
		directAudioFailures = b.audioCompatibility(r, s, container, supported, "", true, false)
	}
	d.Reasons |= directAudioFailures
	if audio != nil && supported == nil {
		d.Reasons |= AudioCodecNotSupported
	}
	var directAudio *core.MediaStream
	if supported != nil && !channelsExceeded && directAudioFailures == 0 && d.Reasons&ContainerBitrateExceedsLimit == 0 {
		directAudio = supported
	}
	if channelsExceeded && d.TargetAudioStream() != nil {
		d.Reasons |= AudioChannelsNotSupported
	}
	d.AudioCodecs = audioCodecs
	if directAudio != nil {
		audio = directAudio
		d.AudioStreamIndex = ptr(audio.Index)
		d.AudioCodecs = []string{audio.Codec}
		d.AudioSampleRate = audio.SampleRate
		channels := ""
		if audio.Channels != 0 {
			channels = strconv.Itoa(audio.Channels)
		}
		d.setOption(qualifier, "audiochannels", channels)
		if audio.Profile != "" {
			d.setOption(audio.Codec, "profile", strings.ToLower(audio.Profile))
		}
		if audio.Level != 0 {
			d.setOption(audio.Codec, "level", strconv.Itoa(audio.Level))
		}
	}

	facts := newVideoFacts(s, video)
	useSubContainer := d.Protocol == HLS
	// Earlier codec profiles take precedence, so they are applied last.
	for i := len(r.Client.Codecs) - 1; i >= 0; i-- {
		p := &r.Client.Codecs[i]
		if p.Kind != CodecVideo || !p.containsAnyCodec(d.VideoCodecs, container, useSubContainer) || len(facts.failing(p.ApplyConditions)) != 0 {
			continue
		}
		for _, codec := range d.VideoCodecs {
			if p.containsCodec(codec, container, useSubContainer) {
				b.applyConditions(d, p.Conditions, codec, true, true)
			}
		}
	}

	d.GlobalMaxAudioChannels = r.MaxAudioChannels
	if channelsExceeded {
		d.GlobalMaxAudioChannels = d.TranscodingMaxAudioChannels
	}
	audioBitrate := audioBitrateFor(r.maxBitrate(true), d.TargetAudioCodec(), audio, d)
	if d.AudioBitrate == 0 || audioBitrate < d.AudioBitrate {
		d.AudioBitrate = audioBitrate
	}

	// Without an audio stream every property is unknown.
	var afacts audioFacts
	if audio != nil {
		afacts = newAudioFacts(audio, s.isSecondaryAudio(audio))
	}
	for i := len(r.Client.Codecs) - 1; i >= 0; i-- {
		p := &r.Client.Codecs[i]
		if p.Kind != CodecVideoAudio || !p.containsAnyCodec(d.AudioCodecs, container, false) || !allSatisfied(p.ApplyConditions, &afacts, true) {
			continue
		}
		for _, codec := range d.AudioCodecs {
			if p.containsCodec(codec, container, false) {
				b.applyConditions(d, p.Conditions, codec, true, true)
				break
			}
		}
	}

	if limit := r.maxBitrate(false); limit > 0 {
		available := int(min(limit, math.MaxInt32))
		if d.AudioBitrate != 0 {
			available -= d.AudioBitrate
		}
		current := d.VideoBitrate
		if current == 0 {
			current = available
		}
		// At least 64 kbit/s, even when the audio leaves less.
		d.VideoBitrate = max(min(available, current), 64_000)
	}
	b.logger().Debug("stream result", "client", r.Client.Name, "path", s.Path, "method", d.Method, "reasons", d.Reasons)
}

// defaultAudioBitrate is the bitrate audio in codec with channels is
// encoded at.
func defaultAudioBitrate(codec string, channels int) int {
	switch strings.ToLower(codec) {
	case "aac", "mp3", "ac3", "eac3":
		switch {
		case channels < 2:
			return 128_000
		case channels >= 6:
			return 640_000
		}
		return 384_000
	case "flac", "alac":
		switch {
		case channels < 2:
			return 768_000
		case channels >= 6:
			return 3_584_000
		}
		return 1_536_000
	}
	return 192_000
}

func audioBitrateFor(maxTotal int64, targetCodecs []string, audio *core.MediaStream, d *Decision) int {
	var target string
	if len(targetCodecs) > 0 {
		target = targetCodecs[0]
	}
	targetChannels := d.targetAudioChannels(target)
	bitrate := 192_000
	encoderLimit := math.MaxInt
	if audio != nil {
		switch {
		case targetChannels.ok && audio.Channels != 0 && audio.Channels > targetChannels.v:
			// Downmixing needs less.
			bitrate = defaultAudioBitrate(target, targetChannels.v)
		case targetChannels.ok && audio.Channels != 0 && audio.Channels <= targetChannels.v &&
			audio.Codec != "" && len(targetCodecs) > 0 && !slices.ContainsFunc(targetCodecs, func(c string) bool { return strings.EqualFold(c, audio.Codec) }):
			// Another codec needs its own rate.
			bitrate = defaultAudioBitrate(target, audio.Channels)
		case audio.Bitrate > 0:
			bitrate = int(audio.Bitrate)
		default:
			bitrate = defaultAudioBitrate(target, targetChannels.v)
		}
		// Encoders fail above 64 kbit/s for mono sources below it.
		if audio.Channels == 1 && audio.Bitrate < 64_000 {
			encoderLimit = 64_000
		}
	}
	if maxTotal > 0 {
		bitrate = min(maxAudioBitrateForTotal(maxTotal), bitrate)
	}
	return min(bitrate, encoderLimit)
}

func maxAudioBitrateForTotal(total int64) int {
	switch {
	case total <= 640_000:
		return 128_000
	case total <= 2_000_000:
		return 384_000
	case total <= 3_000_000:
		return 448_000
	case total <= 4_000_000:
		return 640_000
	case total <= 5_000_000:
		return 768_000
	case total <= 10_000_000:
		return 1_536_000
	case total <= 15_000_000:
		return 2_304_000
	case total <= 20_000_000:
		return 3_584_000
	}
	return 7_168_000
}

// applyConditions narrows the output so that it satisfies conditions;
// qualifier names the codec per-codec conditions apply to.
func (b *Builder) applyConditions(d *Decision, conds []Condition, qualifier string, qualified, unqualified bool) {
	enabled := func() bool {
		if qualifier == "" {
			return unqualified
		}
		return qualified
	}
	narrowInt := func(c Condition, cur opt[int]) (int, bool) {
		n, err := strconv.Atoi(strings.TrimSpace(c.Value))
		if err != nil {
			return 0, false
		}
		switch c.Op {
		case Equals:
			return n, true
		case LessThanEqual:
			if cur.ok {
				return min(n, cur.v), true
			}
			return n, true
		}
		return 0, false
	}
	setList := func(c Condition, name string) {
		var values []string
		for v := range strings.SplitSeq(c.Value, "|") {
			if v = strings.TrimSpace(v); v != "" {
				values = append(values, v)
			}
		}
		switch c.Op {
		case Equals:
			d.setOption(qualifier, name, strings.Join(values, ","))
		case EqualsAny:
			cur := d.Option(qualifier, name)
			if cur != "" && slices.ContainsFunc(values, func(v string) bool { return strings.EqualFold(v, cur) }) {
				d.setOption(qualifier, name, cur)
			} else {
				d.setOption(qualifier, name, strings.Join(values, ","))
			}
		}
	}
	for _, c := range conds {
		// A minimum cannot be produced.
		if c.Value == "" || c.Op == GreaterThanEqual {
			continue
		}
		switch c.Property {
		case AudioBitrate:
			if n, ok := narrowInt(c, nonZero(d.AudioBitrate)); unqualified && ok {
				d.AudioBitrate = n
			}
		case AudioSampleRate:
			if n, ok := narrowInt(c, nonZero(d.AudioSampleRate)); unqualified && ok {
				d.AudioSampleRate = n
			}
		case AudioChannels:
			if n, ok := narrowInt(c, d.targetAudioChannels(qualifier)); enabled() && ok {
				d.setOption(qualifier, "audiochannels", strconv.Itoa(n))
			}
		case IsAVC:
			if v, ok := parseBool(c.Value); unqualified && ok && ((v && c.Op == Equals) || (!v && c.Op == NotEquals)) {
				d.RequireAVC = true
			}
		case IsAnamorphic:
			if v, ok := parseBool(c.Value); unqualified && ok && ((v && c.Op == Equals) || (!v && c.Op == NotEquals)) {
				d.RequireNonAnamorphic = true
			}
		case IsInterlaced:
			if v, ok := parseBool(c.Value); enabled() && ok && ((!v && c.Op == Equals) || (v && c.Op == NotEquals)) {
				d.setOption(qualifier, "deinterlace", "true")
			}
		case RefFrames:
			if n, ok := narrowInt(c, d.intOption(qualifier, "maxrefframes")); enabled() && ok {
				d.setOption(qualifier, "maxrefframes", strconv.Itoa(n))
			}
		case VideoBitDepth:
			if n, ok := narrowInt(c, d.intOption(qualifier, "videobitdepth")); enabled() && ok {
				d.setOption(qualifier, "videobitdepth", strconv.Itoa(n))
			}
		case VideoProfile:
			if qualifier == "" {
				continue
			}
			// Profiles keep their spaces, unlike the other lists.
			values := slices.DeleteFunc(strings.Split(c.Value, "|"), func(v string) bool { return v == "" })
			switch c.Op {
			case Equals:
				d.setOption(qualifier, "profile", strings.Join(values, ","))
			case EqualsAny:
				if cur := d.Option(qualifier, "profile"); cur != "" && slices.Contains(values, cur) {
					d.setOption(qualifier, "profile", cur)
				} else {
					d.setOption(qualifier, "profile", strings.Join(values, ","))
				}
			}
		case VideoRangeType:
			if qualifier == "" {
				continue
			}
			if c.Op == NotEquals {
				var keep []string
				for _, t := range rangeTypes {
					if !slices.ContainsFunc(strings.Split(c.Value, "|"), func(v string) bool { return strings.TrimSpace(v) == string(t) }) {
						keep = append(keep, string(t))
					}
				}
				d.setOption(qualifier, "rangetype", strings.Join(keep, ","))
				continue
			}
			setList(c, "rangetype")
		case VideoCodecTag:
			if qualifier != "" {
				setList(c, "codectag")
			}
		case VideoRotation:
			if qualifier != "" {
				setList(c, "rotation")
			}
		case Height:
			if n, ok := narrowInt(c, nonZero(d.MaxHeight)); unqualified && ok {
				d.MaxHeight = n
			}
		case VideoBitrate:
			if n, ok := narrowInt(c, nonZero(d.VideoBitrate)); unqualified && ok {
				d.VideoBitrate = n
			}
		case VideoFramerate:
			f, err := strconv.ParseFloat(strings.TrimSpace(c.Value), 32)
			if !unqualified || err != nil {
				continue
			}
			switch c.Op {
			case Equals:
				d.MaxFramerate = float32(f)
			case LessThanEqual:
				if d.MaxFramerate == 0 || float32(f) < d.MaxFramerate {
					d.MaxFramerate = float32(f)
				}
			}
		case VideoLevel:
			if qualifier == "" {
				continue
			}
			n, err := strconv.Atoi(strings.TrimSpace(c.Value))
			if err != nil {
				continue
			}
			level := float64(n)
			if cur, ok := parseFloat(d.Option(qualifier, "level")); ok && c.Op == LessThanEqual {
				level = min(level, cur)
			}
			if c.Op == Equals || c.Op == LessThanEqual {
				d.setOption(qualifier, "level", strconv.FormatFloat(level, 'f', -1, 64))
			}
		case Width:
			if n, ok := narrowInt(c, nonZero(d.MaxWidth)); unqualified && ok {
				d.MaxWidth = n
			}
		}
	}
}

// audio decides how to play an audio-only source.
func (b *Builder) audio(s *Source, r *Request) *Decision {
	d := newDecision(s, Audio, r)
	if r.ForceDirectPlay || r.ForceDirectStream {
		d.Method = DirectPlay
		if r.ForceDirectStream && !r.ForceDirectPlay {
			d.Method = DirectStream
		}
		d.Container = singleContainer(s.Container, r.Client, Audio, nil)
		return d
	}
	audio := s.defaultAudioStream(nil)
	if audio == nil {
		return nil
	}
	m := b.audioDirectPlay(s, audio, r)
	reasons := m.reasons
	if m.method == DirectPlay {
		failures := b.audioCompatibility(r, s, s.Container, audio, "", false, false)
		reasons |= failures
		if failures == 0 {
			d.Method = DirectPlay
			d.Container = singleContainer(s.Container, r.Client, Audio, m.profile)
			return d
		}
	}
	if m.method == DirectStream {
		// A container the client named for HLS overrides the default; the
		// client must make sure it fits.
		container := "ts"
		if m.profile != nil && (strings.EqualFold(m.profile.Container.String(), "ts") || strings.EqualFold(m.profile.Container.String(), "mp4")) {
			container = m.profile.Container.String()
		}
		d.Method = DirectStream
		d.Container = container
		d.Reasons = reasons
		d.Protocol = HTTP
		return d
	}

	var tp *TranscodingProfile
	for i := range r.Client.Transcoding {
		p := &r.Client.Transcoding[i]
		codec := p.AudioCodec.String()
		if codec == "" {
			codec = p.Container
		}
		if p.Kind == Audio && p.Context == r.Context && b.Transcoder != nil && b.Transcoder.CanEncodeAudio(codec) {
			tp = p
			break
		}
	}
	if tp != nil {
		if s.NoTranscoding {
			return nil
		}
		b.applyTranscodingProfile(d, tp)
		facts := newAudioFacts(audio, opt[bool]{})
		var conds []Condition
		for i := range r.Client.Codecs {
			p := &r.Client.Codecs[i]
			// All conditions apply, satisfied or not, so that the transcode
			// keeps satisfying them.
			if p.Kind == CodecAudio && p.containsCodec(tp.AudioCodec.String(), tp.Container, false) && allSatisfied(p.ApplyConditions, &facts, false) {
				conds = append(conds, p.Conditions...)
			}
		}
		b.applyConditions(d, conds, "", true, true)
		d.GlobalMaxAudioChannels = r.MaxAudioChannels

		configured := r.maxBitrate(true)
		bitrate := r.AudioTranscodingBitrate
		if bitrate == 0 && r.Context == Streaming {
			bitrate = r.Client.MusicStreamingTranscodingBitrate
		}
		if bitrate == 0 {
			bitrate = configured
		}
		if bitrate == 0 {
			bitrate = 128_000
		}
		if configured > 0 {
			bitrate = min(configured, bitrate)
		}
		if d.AudioBitrate != 0 {
			bitrate = min(bitrate, int64(d.AudioBitrate))
		}
		d.AudioBitrate = int(min(bitrate, math.MaxInt32))
		// Audio transcodes take a single codec.
		if len(d.AudioCodecs) == 0 && strings.TrimSpace(tp.AudioCodec.String()) != "" {
			d.AudioCodecs = []string{tp.AudioCodec.String()}
		}
	}
	d.Reasons = reasons
	return d
}

func (b *Builder) audioDirectPlay(s *Source, audio *core.MediaStream, r *Request) directPlayMatch {
	var played *DirectPlayProfile
	for i := range r.Client.DirectPlay {
		if p := &r.Client.DirectPlay[i]; p.Kind == Audio && audioDirectPlaySupported(p, s, audio) {
			played = p
			break
		}
	}
	var reasons Reasons
	if played == nil {
		b.logger().Debug("no audio direct play profile", "client", r.Client.Name, "path", s.Path, "codec", audio.Codec)
		for i := range r.Client.DirectPlay {
			if p := &r.Client.DirectPlay[i]; p.Kind == Audio && audioDirectStreamSupported(p, s, audio) {
				played = p
				break
			}
		}
		if played == nil {
			return directPlayMatch{reasons: directPlayReasons(s, nil, audio, r.Client.DirectPlay)}
		}
		reasons |= ContainerNotSupported
	}
	exceeded := b.bitrateExceeded(s, r.maxBitrate(true))
	if !s.NoDirectPlay && reasons == 0 {
		if !exceeded {
			if r.EnableDirectPlay {
				return directPlayMatch{profile: played, method: DirectPlay}
			}
		} else {
			reasons |= ContainerBitrateExceedsLimit
		}
	}
	if !s.NoDirectStream {
		if !exceeded {
			// Direct streaming audio is always possible once the codec
			// fits, whatever EnableDirectStream says.
			if reasons == ContainerNotSupported {
				return directPlayMatch{profile: played, method: DirectStream, reasons: reasons}
			}
		} else {
			reasons |= ContainerBitrateExceedsLimit
		}
	}
	return directPlayMatch{profile: played, reasons: reasons}
}

// directPlayReasons explains why no direct play profile fits.
func directPlayReasons(s *Source, video, audio *core.MediaStream, profiles []DirectPlayProfile) Reasons {
	kind := Audio
	if video != nil {
		kind = Video
	}
	// Jellyfin also blames the video codec of audio files no profile
	// takes; only streams that exist are blamed here.
	containerOK, audioOK, videoOK := false, audio == nil, video == nil
	for i := range profiles {
		p := &profiles[i]
		if p.Kind != kind || !p.supportsContainer(s.Container) {
			continue
		}
		containerOK = true
		videoOK = video == nil || p.supportsVideoCodec(video.Codec)
		audioOK = audio == nil || p.supportsAudioCodec(audio.Codec)
		if videoOK && audioOK {
			break
		}
	}
	var r Reasons
	if !containerOK {
		r |= ContainerNotSupported
	}
	if !videoOK {
		r |= VideoCodecNotSupported
	}
	if !audioOK {
		r |= AudioCodecNotSupported
	}
	return r
}

func audioContainerSupported(p *DirectPlayProfile, s *Source) bool {
	if !p.supportsContainer(s.Container) {
		return false
	}
	// Matroska is never played on clients that only declare WebM, which
	// the first check lets through.
	return !Names("mkv").Contains(s.Container) || p.supportsContainer("mkv")
}

func audioDirectPlaySupported(p *DirectPlayProfile, s *Source, audio *core.MediaStream) bool {
	return audioContainerSupported(p, s) && p.supportsAudioCodec(audio.Codec)
}

// audioDirectStreamSupported reports whether the codec can be remuxed into
// a container the profile names: the container itself must not fit, and
// the profile must name the codec exactly.
func audioDirectStreamSupported(p *DirectPlayProfile, s *Source, audio *core.MediaStream) bool {
	if audioContainerSupported(p, s) {
		return false
	}
	return strings.EqualFold(p.AudioCodec.String(), audio.Codec) || strings.EqualFold(p.Container.String(), audio.Codec)
}
