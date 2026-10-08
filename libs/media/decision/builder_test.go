package decision

import (
	"slices"
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

// fakeTranscoder extracts subtitles when asked to and encodes no audio, as
// the mock in Jellyfin's tests does.
type fakeTranscoder struct{ extract, encode bool }

func (f fakeTranscoder) CanEncodeAudio(string) bool      { return f.encode }
func (f fakeTranscoder) CanExtractSubtitles(string) bool { return f.extract }

var methods = map[string]Method{"DirectPlay": DirectPlay, "DirectStream": DirectStream, "Transcode": Transcode}

var subtitleMethods = map[string]SubtitleMethod{
	"Encode": SubtitleEncode, "Embed": SubtitleEmbed, "External": SubtitleExternal, "Hls": SubtitleHLS, "Drop": SubtitleDrop,
}

func videoRequest(t *testing.T, client, source string) *Request {
	t.Helper()
	s := loadSource(t, source)
	return &Request{
		Sources:              []*Source{s},
		SourceID:             s.ID,
		Client:               loadProfile(t, client),
		Context:              Streaming,
		EnableDirectPlay:     true,
		EnableDirectStream:   false, // disabled in the server
		AllowAudioStreamCopy: true,
		AllowVideoStreamCopy: true,
	}
}

// location returns the file name and extension of the URL a decision is
// served at: the HLS master playlist or the progressive stream.
func location(d *Decision) (string, string) {
	if d.Protocol == HLS {
		return "master", "m3u8"
	}
	return "stream", d.Container
}

func streamsOf(s *Source, kind core.StreamKind) []*core.MediaStream {
	var out []*core.MediaStream
	for i := range s.Streams {
		if s.Streams[i].Kind == kind {
			out = append(out, &s.Streams[i])
		}
	}
	return out
}

// checkVideoDecision ports BuildVideoItemSimpleTest.
func checkVideoDecision(t *testing.T, r *Request, a args) *Decision {
	t.Helper()
	method, hasMethod := methods[a.symbol(t, "playMethod")]
	why := a.reasons(t, "why")
	mode := a.str(t, "transcodeMode")
	protocol := a.str(t, "transcodeProtocol")
	if protocol == "" {
		protocol = "HLS.ts"
	}
	b := &Builder{Transcoder: fakeTranscoder{}}
	d, err := b.Video(r)
	if err != nil {
		t.Fatal(err)
	}
	if d == nil {
		t.Fatal("got = no decision")
	}
	if hasMethod && d.Method != method {
		t.Errorf("method: got = %s, want = %s", d.Method, method)
	}
	if d.Reasons != why {
		t.Errorf("reasons: got = %v, want = %v", d.Reasons, why)
	}
	s := d.Source
	video, audio := d.TargetVideoStream(), d.TargetAudioStream()
	name, ext := location(d)

	switch {
	case hasMethod && method == DirectPlay:
		if !slices.Contains(strings.Split(s.Container, ","), ext) {
			t.Errorf("container: got = %s, want one of %s", ext, s.Container)
		}
		if video != nil && video.Codec != "" && !slices.Equal(d.TargetVideoCodec(), []string{video.Codec}) {
			t.Errorf("video codec: got = %v, want = %s", d.TargetVideoCodec(), video.Codec)
		}
		if audio != nil && audio.Codec != "" && !slices.Equal(d.TargetAudioCodec(), []string{audio.Codec}) {
			t.Errorf("audio codec: got = %v, want = %s", d.TargetAudioCodec(), audio.Codec)
		}
	case hasMethod && method == Transcode:
		if d.Container == "" || len(d.VideoCodecs) == 0 || len(d.AudioCodecs) == 0 {
			t.Errorf("output: got = %q %v %v, want all set", d.Container, d.VideoCodecs, d.AudioCodecs)
		}
		switch protocol {
		case "http":
			if ext != d.Container || name != "stream" || d.Protocol != HTTP {
				t.Errorf("http: got = %s.%s over %s", name, ext, d.Protocol)
			}
		case "HLS.mp4":
			if d.Container != "mp4" || name != "master" || d.Protocol != HLS {
				t.Errorf("HLS.mp4: got = %s in %s.%s over %s", d.Container, name, ext, d.Protocol)
			}
		default:
			if d.Container != "ts" || name != "master" || d.Protocol != HLS {
				t.Errorf("HLS.ts: got = %s in %s.%s over %s", d.Container, name, ext, d.Protocol)
			}
		}
		if mode == "Transcode" {
			if d.Reasons&(containerReasons|DirectPlayError|VideoRangeTypeNotSupported) == 0 {
				for _, v := range streamsOf(s, core.StreamVideo) {
					if slices.Contains(d.VideoCodecs, v.Codec) {
						t.Errorf("video codecs: got = %v, want without %s", d.VideoCodecs, v.Codec)
					}
				}
			}
			break
		}
		// Direct stream and remux copy the video.
		if video == nil || !slices.Equal(d.TargetVideoCodec(), []string{video.Codec}) {
			t.Errorf("target video codec: got = %v", d.TargetVideoCodec())
		}
		switch mode {
		case "DirectStream":
			if audio != nil && !isExternal(audio) {
				if why&AudioChannelsNotSupported == 0 {
					if slices.Contains(d.AudioCodecs, audio.Codec) {
						t.Errorf("audio codecs: got = %v, want without %s", d.AudioCodecs, audio.Codec)
					}
				} else if d.TargetAudioChannels() != d.TranscodingMaxAudioChannels {
					// Jellyfin rewrites the source stream's channels; Mavio
					// leaves the source alone and caps the target.
					t.Errorf("audio channels: got = %d, want = %d", d.TargetAudioChannels(), d.TranscodingMaxAudioChannels)
				}
			}
		case "Remux":
			if audio == nil || !slices.Equal(d.AudioCodecs, []string{audio.Codec}) {
				t.Errorf("audio codecs: got = %v, want the source's", d.AudioCodecs)
			}
		}
		if d.EstimateContentLength || d.SeekInfo != SeekAuto {
			t.Errorf("seek: got = %v %s", d.EstimateContentLength, d.SeekInfo)
		}
		var profiles []string
		for p := range strings.SplitSeq(d.TargetVideoProfile(), ",") {
			profiles = append(profiles, strings.ToLower(p))
		}
		if d.TargetVideoProfile() == "" {
			profiles = nil
		}
		if !slices.Contains(profiles, strings.ToLower(video.Profile)) {
			t.Errorf("video profile: got = %q, want %q", d.TargetVideoProfile(), video.Profile)
		}
		if d.TargetVideoLevel() != float64(video.Level) {
			t.Errorf("video level: got = %v, want = %d", d.TargetVideoLevel(), video.Level)
		}
		if d.TargetVideoBitDepth() != video.BitDepth {
			t.Errorf("video bit depth: got = %d, want = %d", d.TargetVideoBitDepth(), video.BitDepth)
		}
		if int64(d.VideoBitrate) < video.Bitrate {
			t.Errorf("video bitrate: got = %d, want >= %d", d.VideoBitrate, video.Bitrate)
		}
		if why&AudioCodecNotSupported != 0 {
			if r.AudioStreamIndex != nil && *r.AudioStreamIndex >= 0 {
				if audio != nil && !isExternal(audio) && slices.Contains(d.AudioCodecs, audio.Codec) {
					t.Errorf("audio codecs: got = %v, want without %s", d.AudioCodecs, audio.Codec)
				}
			} else {
				for _, st := range streamsOf(s, core.StreamAudio) {
					isDefault := audio != nil && audio.Default
					candidate := (isDefault && st.Default) || (!isDefault && audio != nil && st.Language == audio.Language)
					if candidate && !isExternal(st) && slices.Contains(d.AudioCodecs, st.Codec) {
						t.Errorf("audio codecs: got = %v, want without %s", d.AudioCodecs, st.Codec)
					}
				}
			}
		}
	case !hasMethod:
		if d.Protocol != HTTP || name != "stream" || d.EstimateContentLength || d.SeekInfo != SeekAuto {
			t.Errorf("got = %s %s %v %s", d.Protocol, name, d.EstimateContentLength, d.SeekInfo)
		}
	}
	return d
}

func intPtrEqual(a, b *int) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }

func TestStreamBuilderCases(t *testing.T) {
	request := func(t *testing.T, a args) *Request {
		return videoRequest(t, a.str(t, "deviceName"), a.str(t, "mediaSource"))
	}
	portedCases(t, "stream_builder.json", ported{
		run: map[string]func(t *testing.T, a args){
			"BuildVideoItemSimple": func(t *testing.T, a args) {
				checkVideoDecision(t, request(t, a), a)
			},
			"BuildVideoItemWithFirstExplicitStream": func(t *testing.T, a args) {
				r := request(t, a)
				r.AudioStreamIndex = ptr(1)
				r.SubtitleStreamIndex = ptr(len(r.Sources[0].Streams) - 1)
				d := checkVideoDecision(t, r, a)
				if !intPtrEqual(d.AudioStreamIndex, r.AudioStreamIndex) || !intPtrEqual(d.SubtitleStreamIndex, r.SubtitleStreamIndex) {
					t.Errorf("streams: got = %v %v, want = %v %v", d.AudioStreamIndex, d.SubtitleStreamIndex, r.AudioStreamIndex, r.SubtitleStreamIndex)
				}
			},
			"BuildVideoItemWithDirectPlayExplicitStreams": func(t *testing.T, a args) {
				r := request(t, a)
				if n := len(r.Sources[0].Streams); n > 0 {
					r.AudioStreamIndex = ptr(n - 2)
					r.SubtitleStreamIndex = ptr(n - 1)
				}
				d := checkVideoDecision(t, r, a)
				if !intPtrEqual(d.AudioStreamIndex, r.AudioStreamIndex) || !intPtrEqual(d.SubtitleStreamIndex, r.SubtitleStreamIndex) {
					t.Errorf("streams: got = %v %v, want = %v %v", d.AudioStreamIndex, d.SubtitleStreamIndex, r.AudioStreamIndex, r.SubtitleStreamIndex)
				}
			},
			"BuildVideoItemWithSecondaryAudioAndExternalGraphicalSubtitleKeepsVideoCopy": func(t *testing.T, a args) {
				r := videoRequest(t, "Chrome", "mp4-h264-ac3-aac-srt-2600k")
				s := r.Sources[0]
				sub := &s.Streams[len(s.Streams)-1]
				sub.Codec = a.str(t, "subtitleCodec")
				sub.ExternalPath = ""
				var container Names
				if !a.null("subtitleContainer") {
					container = Names(a.str(t, "subtitleContainer"))
				}
				r.Client.Subtitles = []SubtitleProfile{{Format: sub.Codec, Container: container, Method: SubtitleExternal}}
				r.AudioStreamIndex = ptr(2)
				r.SubtitleStreamIndex = ptr(sub.Index)
				d, err := (&Builder{Transcoder: fakeTranscoder{}}).Video(r)
				if err != nil {
					t.Fatal(err)
				}
				if d.Method != Transcode || d.Reasons != SecondaryAudioNotSupported || d.SubtitleMethod != SubtitleExternal {
					t.Errorf("got = %s %v %s", d.Method, d.Reasons, d.SubtitleMethod)
				}
				if !slices.Contains(d.VideoCodecs, "h264") || !slices.Contains(d.AudioCodecs, "aac") {
					t.Errorf("codecs: got = %v %v", d.VideoCodecs, d.AudioCodecs)
				}
				// An external subtitle is not handed to the transcoder.
				if d.AlwaysBurnInSubtitleWhenTranscoding || d.SubtitleMethod != SubtitleExternal {
					t.Error("subtitle would be passed to the transcoder")
				}
			},
			"GetSubtitleProfile_RespectsExtractionSetting": func(t *testing.T, a args) {
				codec := a.str(t, "codec")
				st := core.MediaStream{Kind: core.StreamSubtitle, Codec: codec}
				if a.boolean(t, "isExternal") {
					st.ExternalPath = "/media/sub." + codec
				}
				profiles := []SubtitleProfile{{Format: a.str(t, "profileFormat"), Method: SubtitleExternal}}
				got := SubtitleProfileFor(&Source{MediaSource: &core.MediaSource{}}, &st, profiles, methods[a.symbol(t, "playMethod")],
					fakeTranscoder{extract: a.boolean(t, "enableSubtitleExtraction")}, "", "")
				if want := subtitleMethods[a.symbol(t, "expectedMethod")]; got.Method != want {
					t.Errorf("got = %s, want = %s", got.Method, want)
				}
			},
			"GetSubtitleProfile_MatchesVobSubMksProfileOnlyWhenDeliveredAsMks": func(t *testing.T, a args) {
				st := core.MediaStream{Kind: core.StreamSubtitle, Codec: "vobsub"}
				if a.boolean(t, "isExternal") {
					st.ExternalPath = "external"
					if !a.null("path") {
						st.ExternalPath = a.str(t, "path")
					}
				}
				profiles := []SubtitleProfile{{Format: "vobsub", Container: "mks", Method: SubtitleExternal}}
				got := SubtitleProfileFor(&Source{MediaSource: &core.MediaSource{}}, &st, profiles, Transcode,
					fakeTranscoder{extract: a.boolean(t, "enableSubtitleExtraction")}, "", "")
				if want := subtitleMethods[a.symbol(t, "expectedMethod")]; got.Method != want {
					t.Errorf("got = %s, want = %s", got.Method, want)
				}
			},
			"GetSubtitleProfile_ReturnsExpectedDeliveryMethod": func(t *testing.T, a args) {
				codec := a.str(t, "codec")
				st := core.MediaStream{Kind: core.StreamSubtitle, Codec: codec, Language: "eng"}
				if a.boolean(t, "isExternal") {
					st.ExternalPath = "external"
				}
				profiles := []SubtitleProfile{{Format: codec, Method: SubtitleEmbed}, {Format: codec, Method: SubtitleExternal}}
				got := SubtitleProfileFor(&Source{MediaSource: &core.MediaSource{}}, &st, profiles, methods[a.symbol(t, "playMethod")],
					fakeTranscoder{extract: true}, a.str(t, "outputContainer"), Protocol(a.symbol(t, "transcodingSubProtocol")))
				if want := subtitleMethods[a.symbol(t, "expectedMethod")]; got.Method != want {
					t.Errorf("got = %s, want = %s", got.Method, want)
				}
			},
		},
	})
}

func manifestDecision(t *testing.T, container string) *Decision {
	t.Helper()
	s := &Source{
		MediaSource: &core.MediaSource{
			ID: sourceID("test-source"), Path: "http://example.com/live/channel", Container: container,
			Streams: []core.MediaStream{{Kind: core.StreamVideo, Index: 0, Codec: "h264"}, {Kind: core.StreamAudio, Index: 1, Codec: "aac"}},
		},
		Remote: true, Infinite: true,
	}
	client := &ClientCapabilities{
		Name:                "Manifest aware client",
		MaxStreamingBitrate: 8_000_000, MaxStaticBitrate: 8_000_000,
		DirectPlay:  []DirectPlayProfile{{Kind: Video, Container: "mp4,hls,applehttp,dash", VideoCodec: "h264", AudioCodec: "aac"}},
		Transcoding: []TranscodingProfile{{Kind: Video, Context: Streaming, Protocol: HLS, Container: "ts", VideoCodec: "h264", AudioCodec: "aac"}},
	}
	d, err := (&Builder{Transcoder: fakeTranscoder{}}).Video(&Request{
		Sources: []*Source{s}, SourceID: s.ID, Client: client, Context: Streaming,
		EnableDirectPlay: true, AllowAudioStreamCopy: true, AllowVideoStreamCopy: true,
	})
	if err != nil || d == nil {
		t.Fatalf("got = %v, %v", d, err)
	}
	return d
}

func TestStreamBuilderManifestContainerCases(t *testing.T) {
	portedCases(t, "stream_builder_manifest_container.json", ported{
		run: map[string]func(t *testing.T, a args){
			"GetOptimalVideoStream_ManifestContainer_DoesNotDirectPlay": func(t *testing.T, a args) {
				if d := manifestDecision(t, a.str(t, "container")); d.Method != Transcode {
					t.Errorf("got = %s, want = %s", d.Method, Transcode)
				}
			},
		},
		facts: map[string]string{"GetOptimalVideoStream_ByteStreamContainer_StillDirectPlays": "TestByteStreamContainerDirectPlays"},
	})
}

func TestByteStreamContainerDirectPlays(t *testing.T) {
	if d := manifestDecision(t, "mp4"); d.Method != DirectPlay {
		t.Errorf("got = %s, want = %s", d.Method, DirectPlay)
	}
}

func TestStreamInfoCases(t *testing.T) {
	const reason = "compares Jellyfin's stream URLs with their legacy form; Mavio's URLs are defined by libs/streaming"
	portedCases(t, "stream_info.json", ported{
		skip: map[string]string{"Test_Blank_Url_Method": reason, "Fuzzy_Comparison": reason},
	})
}

func TestAudioDecision(t *testing.T) {
	audioSource := func(container, codec string, channels int, bitrate int64) *Source {
		return &Source{MediaSource: &core.MediaSource{
			ID: sourceID(container + codec), Container: container, Bitrate: bitrate,
			Streams: []core.MediaStream{{Kind: core.StreamAudio, Codec: codec, Channels: channels, Bitrate: bitrate}},
		}}
	}
	tests := []struct {
		name      string
		source    *Source
		method    Method
		reasons   Reasons
		container string
		codecs    []string
	}{
		{"flac plays", audioSource("flac", "flac", 2, 1_000_000), DirectPlay, 0, "flac", nil},
		{"aac in mp4 remuxes", audioSource("mov,mp4,m4a", "aac", 2, 256_000), DirectPlay, 0, "m4a", nil},
		{"mkv is not webm", audioSource("mkv", "opus", 2, 128_000), DirectStream, ContainerNotSupported, "ts", nil},
		{"wma transcodes", audioSource("asf", "wmav2", 2, 192_000), Transcode, ContainerNotSupported | AudioCodecNotSupported, "mp4", []string{"aac"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Request{
				Sources: []*Source{tt.source}, Client: loadProfile(t, "Chrome"), Context: Streaming,
				EnableDirectPlay: true, AllowAudioStreamCopy: true,
			}
			d, err := (&Builder{Transcoder: fakeTranscoder{encode: true}}).Audio(r)
			if err != nil || d == nil {
				t.Fatalf("got = %v, %v", d, err)
			}
			if d.Method != tt.method || d.Reasons != tt.reasons || d.Container != tt.container || !slices.Equal(d.AudioCodecs, tt.codecs) {
				t.Errorf("got = %s %v %s %v, want = %s %v %s %v", d.Method, d.Reasons, d.Container, d.AudioCodecs, tt.method, tt.reasons, tt.container, tt.codecs)
			}
			if d.Method == Transcode && (d.Protocol != HLS || d.AudioBitrate != 384_000 || d.TranscodingMaxAudioChannels != 2) {
				t.Errorf("transcode: got = %s %d %d", d.Protocol, d.AudioBitrate, d.TranscodingMaxAudioChannels)
			}
		})
	}
}

func TestRequestValidation(t *testing.T) {
	b := &Builder{}
	if _, err := b.Video(&Request{}); err == nil {
		t.Error("no client: got = nil error")
	}
	if _, err := b.Video(&Request{Client: &ClientCapabilities{}, AudioStreamIndex: ptr(1)}); err == nil {
		t.Error("stream without source: got = nil error")
	}
}
