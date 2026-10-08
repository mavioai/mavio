package probe

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func (a args) opt(t *testing.T, name string, v any) bool {
	t.Helper()
	raw, ok := a[name]
	if !ok || string(raw) == "null" {
		return false
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("argument %s: %v", name, err)
	}
	return true
}

func TestPortedProbeResultNormalizer(t *testing.T) {
	facts := map[string]string{}
	for name := range probeFacts {
		facts[name] = "TestNormalize"
	}
	portedCases(t, "probe_result_normalizer.json", ported{
		run: map[string]func(*testing.T, args){
			"GetFrameRate_Success": func(t *testing.T, a args) {
				var want float32
				hasWant := a.opt(t, "expected", &want)
				got, ok := FrameRate(a.str(t, "value"))
				if ok != hasWant || got != want {
					t.Errorf("FrameRate: got = %v %v, want = %v %v", got, ok, want, hasWant)
				}
			},
			"IsNearSquarePixelSar_DetectsCorrectly": func(t *testing.T, a args) {
				var sar string
				var want bool
				a.opt(t, "sar", &sar)
				a.opt(t, "expected", &want)
				if got := IsNearSquarePixelSAR(sar); got != want {
					t.Errorf("IsNearSquarePixelSAR(%q): got = %v, want = %v", sar, got, want)
				}
			},
			"GetEstimatedAudioBitrate_ReturnsExpected": func(t *testing.T, a args) {
				var profile string
				var channels, want int
				a.opt(t, "profile", &profile)
				a.opt(t, "channels", &channels)
				hasWant := a.opt(t, "expected", &want)
				got, ok := EstimatedAudioBitrate(a.str(t, "codec"), profile, channels)
				if ok != hasWant || got != want {
					t.Errorf("got = %d %v, want = %d %v", got, ok, want, hasWant)
				}
			},
		},
		facts: facts,
	})
}

func TestPortedProbeExternalSources(t *testing.T) {
	portedCases(t, "probe_external_sources.json", ported{facts: map[string]string{
		"GetExtraArguments_Forwards_UserAgent": "TestProbeArgs",
	}})
}

func TestProbeArgs(t *testing.T) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"
	p := &Prober{FFprobe: "ffprobe", ProbeSize: "5000000", FirstVideoFrame: true}
	args := p.Args(Request{Path: "/path/to/stream", Remote: true, UserAgent: ua})
	if i := slices.Index(args, "-user_agent"); i < 0 || args[i+1] != ua {
		t.Errorf("user agent: got = %q", args)
	}
	if slices.Contains(args, "-show_frames") {
		t.Errorf("remote: got = %q, want no frames", args)
	}
	local := p.Args(Request{Path: "/media/a.mkv", Chapters: true, AnalyzeDuration: 2 * time.Second})
	want := []string{
		"-analyzeduration", "2000000", "-probesize", "5000000", "-i", "file:/media/a.mkv", "-threads", "0",
		"-v", "warning", "-print_format", "json", "-show_streams", "-show_format", "-show_chapters", "-show_frames", "-only_first_vframe",
	}
	if !slices.Equal(local, want) {
		t.Errorf("local: got = %q, want = %q", local, want)
	}
}

func load(t *testing.T, name string, isAudio bool) Result {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "probe", name))
	if err != nil {
		t.Fatal(err)
	}
	var out Output
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return Normalize(&out, name, isAudio)
}

func check[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got = %v, want = %v", what, got, want)
	}
}

func video(t *testing.T, r Result) core.MediaStream {
	t.Helper()
	for _, s := range r.Source.Streams {
		if s.Kind == core.StreamVideo {
			return s
		}
	}
	t.Fatal("video stream: got = none")
	return core.MediaStream{}
}

// checkVideo checks the fields Jellyfin's tests assert for a video stream.
func checkVideo(t *testing.T, v core.MediaStream, w videoWant) {
	t.Helper()
	check(t, "index", v.Index, 0)
	check(t, "codec", v.Codec, "h264")
	check(t, "profile", v.Profile, w.profile)
	check(t, "height", v.Height, w.height)
	check(t, "width", v.Width, w.width)
	check(t, "interlaced", v.Interlaced, w.interlaced)
	check(t, "aspect ratio", v.AspectRatio, w.aspect)
	check(t, "pixel format", v.PixelFormat, "yuv420p")
	check(t, "level", v.Level, w.level)
	check(t, "ref frames", v.RefFrames, 1)
	check(t, "avc", v.AVC, true)
	check(t, "real frame rate", v.RealFrameRate.Float32(), w.realFPS)
	check(t, "time base", v.TimeBase, w.timeBase)
	check(t, "bitrate", v.Bitrate, w.bitrate)
	check(t, "bit depth", v.BitDepth, 8)
	check(t, "default", v.Default, true)
}

type videoWant struct {
	profile       string
	width, height int
	interlaced    bool
	aspect        string
	level         int
	realFPS       float32
	timeBase      string
	bitrate       int64
}

var probeFacts = map[string]func(t *testing.T){
	"GetMediaInfo_MetaData_Success": func(t *testing.T) {
		r := load(t, "video_metadata.json", false)
		check(t, "container", r.Source.Container, "mkv")
		check(t, "streams", len(r.Source.Streams), 3)
		v := video(t, r)
		check(t, "aspect ratio", v.AspectRatio, "4:3")
		check(t, "average frame rate", v.FrameRate.Float32(), 25)
		check(t, "bit depth", v.BitDepth, 8)
		// No video bitrate and the other streams exceed the container's,
		// so none can be inferred (jellyfin/jellyfin#16248).
		check(t, "bitrate", v.Bitrate, 0)
		check(t, "codec", v.Codec, "h264")
		check(t, "codec time base", v.CodecTimeBase, "1/50")
		check(t, "size", [2]int{v.Width, v.Height}, [2]int{320, 240})
		check(t, "anamorphic", v.Anamorphic, false)
		check(t, "avc", v.AVC, true)
		check(t, "default", v.Default, true)
		check(t, "flags", v.Forced || v.HearingImpaired || v.Interlaced || v.ExternalPath != "" || v.IsTextSubtitle(), false)
		check(t, "level", v.Level, 13)
		check(t, "nal length size", v.NALLengthSize, "4")
		check(t, "pixel format", v.PixelFormat, "yuv444p")
		check(t, "profile", v.Profile, "High 4:4:4 Predictive")
		check(t, "real frame rate", v.RealFrameRate.Float32(), 25)
		check(t, "ref frames", v.RefFrames, 1)
		check(t, "time base", v.TimeBase, "1/1000")
		if dv := v.DolbyVision; dv == nil || *dv != (core.DolbyVision{Profile: 5, Level: 6, RPUPresent: true, BLPresent: true, VersionMajor: 1}) {
			t.Errorf("dolby vision: got = %+v", v.DolbyVision)
		}
		check(t, "rotation", v.Rotation, -180)
		a1, a2 := r.Source.Streams[1], r.Source.Streams[2]
		check(t, "audio 1 codec", a1.Codec, "eac3")
		check(t, "audio 1 original", a1.Original, true)
		check(t, "audio 1 spatial", a1.SpatialFormat(), core.SpatialDolbyAtmos)
		check(t, "audio 2 codec", a2.Codec, "dts")
		check(t, "audio 2 original", a2.Original, false)
		check(t, "audio 2 spatial", a2.SpatialFormat(), core.SpatialDTSX)
		check(t, "chapters", len(r.Source.Chapters), 0)
		check(t, "overview", r.Metadata.Overview, "Just color bars")
	},
	"GetMediaInfo_Mp4MetaData_Success": func(t *testing.T) {
		r := load(t, "video_mp4_metadata.json", false)
		// Video, audio (main, commentary), subtitles (Spanish, English, commentary).
		check(t, "streams", len(r.Source.Streams), 6)
		v := r.Source.Streams[0]
		check(t, "kind", v.Kind, core.StreamVideo)
		checkVideo(t, v, videoWant{"High", 720, 358, false, "2.40:1", 31, 120, "1/90000", 1147365})
		check(t, "anamorphic", v.Anamorphic, true) // SAR 32:27, NTSC DVD 16:9
		check(t, "language", v.Language, "und")
		s := r.Source.Streams
		check(t, "audio 1", [4]any{s[1].Kind, s[1].Codec, s[1].Channels, s[1].Default}, [4]any{core.StreamAudio, "aac", 7, true})
		check(t, "audio 1 original", s[1].Original, false)
		check(t, "audio 1 language", s[1].Language, "eng")
		check(t, "audio 1 title", s[1].Title, "Surround 6.1")
		check(t, "audio 2", [4]any{s[2].Kind, s[2].Codec, s[2].Channels, s[2].Default}, [4]any{core.StreamAudio, "aac", 2, false})
		check(t, "audio 2 title", s[2].Title, "Commentary")
		check(t, "sub 1", [4]any{s[3].Kind, s[3].Codec, s[3].Language, s[3].Title}, [4]any{core.StreamSubtitle, "DVDSUB", "spa", ""})
		check(t, "sub 1 hi", s[3].HearingImpaired, false)
		check(t, "sub 2", [4]any{s[4].Kind, s[4].Codec, s[4].Language, s[4].Title}, [4]any{core.StreamSubtitle, "mov_text", "eng", "SDH"})
		check(t, "sub 2 hi", s[4].HearingImpaired, true)
		check(t, "sub 3", [4]any{s[5].Kind, s[5].Codec, s[5].Language, s[5].Title}, [4]any{core.StreamSubtitle, "mov_text", "eng", "Commentary"})
		check(t, "sub 3 hi", s[5].HearingImpaired, false)
	},
	"GetMediaInfo_TS_Success": func(t *testing.T) {
		r := load(t, "video_ts.json", false)
		check(t, "streams", len(r.Source.Streams), 2)
		check(t, "avc", r.Source.Streams[0].AVC, false)
	},
	"GetMediaInfo_WebM_Success": func(t *testing.T) {
		r := load(t, "video_webm.json", false)
		check(t, "container", r.Source.Container, "mkv,webm")
		check(t, "streams", len(r.Source.Streams), 2)
		check(t, "size", [2]int{r.Source.Streams[0].Width, r.Source.Streams[0].Height}, [2]int{540, 360})
	},
	"GetMediaInfo_WebM_Like_Mkv": func(t *testing.T) {
		r := load(t, "video_web_like_mkv_with_subtitle.json", false)
		check(t, "container", r.Source.Container, "mkv")
		check(t, "streams", len(r.Source.Streams), 3)
	},
	"GetMediaInfo_ProgressiveVideoNoFieldOrder_Success": func(t *testing.T) {
		r := load(t, "video_progressive_no_field_order.json", false)
		check(t, "streams", len(r.Source.Streams), 2)
		checkVideo(t, r.Source.Streams[0], videoWant{"Main", 1920, 1080, false, "16:9", 41, 23.9760246, "1/24000", 3948341})
	},
	"GetMediaInfo_ProgressiveVideoNoFieldOrder2_Success": func(t *testing.T) {
		r := load(t, "video_progressive_no_field_order2.json", false)
		check(t, "streams", len(r.Source.Streams), 1)
		checkVideo(t, r.Source.Streams[0], videoWant{"High", 1280, 720, false, "16:9", 31, 25, "1/12800", 53288})
	},
	"GetMediaInfo_InterlacedVideo_Success": func(t *testing.T) {
		r := load(t, "video_interlaced.json", false)
		check(t, "streams", len(r.Source.Streams), 1)
		checkVideo(t, r.Source.Streams[0], videoWant{"High", 1280, 720, true, "16:9", 40, 25, "1/12800", 56945})
	},
	// The video bitrate is the container's minus the audio's
	// (jellyfin/jellyfin#16248).
	"GetMediaInfo_MissingVideoBitrate_EstimatedFromContainer": func(t *testing.T) {
		r := load(t, "video_missing_video_bitrate.json", false)
		check(t, "streams", len(r.Source.Streams), 2)
		check(t, "audio bitrate", r.Source.Streams[1].Bitrate, 128000)
		check(t, "video bitrate", video(t, r).Bitrate, 5000000)
		check(t, "container bitrate", r.Source.Bitrate, 5128000)
	},
	// NUMBER_OF_BYTES with a nanosecond DURATION tag.
	"GetMediaInfo_NanosecondDurationTag_BitrateComputedFromBytes": func(t *testing.T) {
		check(t, "video bitrate", video(t, load(t, "video_nanosecond_duration_bitrate.json", false)).Bitrate, 800000)
	},
	"GetMediaInfo_MissingVideoBitrate_UnknownAudioBitrate_NotEstimated": func(t *testing.T) {
		r := load(t, "video_missing_video_bitrate_unknown_audio.json", false)
		check(t, "streams", len(r.Source.Streams), 2)
		check(t, "video bitrate", video(t, r).Bitrate, 0)
		check(t, "audio bitrate", r.Source.Streams[1].Bitrate, 0)
		check(t, "container bitrate", r.Source.Bitrate, 5128000)
	},
	"GetMediaInfo_VideoWithSingleFrameMjpeg_Success": func(t *testing.T) {
		r := load(t, "video_single_frame_mjpeg.json", false)
		check(t, "streams", len(r.Source.Streams), 3)
		v := r.Source.Streams[0]
		check(t, "kind", v.Kind, core.StreamVideo)
		check(t, "profile", v.Profile, "High")
		check(t, "size", [2]int{v.Width, v.Height}, [2]int{1920, 1080})
		check(t, "aspect ratio", v.AspectRatio, "16:9")
		check(t, "level", v.Level, 42)
		check(t, "real frame rate", v.RealFrameRate.Float32(), 50)
		check(t, "time base", v.TimeBase, "1/1000")
		check(t, "mjpeg", r.Source.Streams[2].Codec, "mjpeg")
	},
	"GetMediaInfo_MusicVideo_Success": func(t *testing.T) {
		md := load(t, "music_video_metadata.json", false).Metadata
		check(t, "name", md.Name, "The Title")
		check(t, "sort name", md.SortName, "Title, The")
		check(t, "artists", strings.Join(md.Artists, "|"), "The Artist")
		check(t, "album", md.Album, "Album")
		check(t, "year", md.ProductionYear, 2021)
		if md.PremiereDate == nil || !md.PremiereDate.Equal(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("premiere: got = %v", md.PremiereDate)
		}
	},
	"GetMediaInfo_GivenOriginalDateContainsOnlyYear_Success": func(t *testing.T) {
		md := load(t, "music_year_only_metadata.json", true).Metadata
		check(t, "name", md.Name, "Baker Street")
		check(t, "artists", strings.Join(md.Artists, "|"), "Gerry Rafferty")
		check(t, "album", md.Album, "City to City")
		check(t, "year", md.ProductionYear, 1978)
		if md.PremiereDate == nil || !md.PremiereDate.Equal(time.Date(1978, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("premiere: got = %v", md.PremiereDate)
		}
		for _, g := range []string{"Electronic", "Ambient", "Pop", "Jazz"} {
			if !slices.Contains(md.Genres, g) {
				t.Errorf("genres: got = %q, want %q", md.Genres, g)
			}
		}
	},
	"GetMediaInfo_Music_Success": func(t *testing.T) {
		r := load(t, "music_metadata.json", true)
		md := r.Metadata
		check(t, "name", md.Name, "UP NO MORE")
		check(t, "artists", strings.Join(md.Artists, "|"), "TWICE")
		check(t, "album", md.Album, "Eyes wide open")
		check(t, "year", md.ProductionYear, 2020)
		if md.PremiereDate == nil || !md.PremiereDate.Equal(time.Date(2020, 10, 26, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("premiere: got = %v", md.PremiereDate)
		}
		check(t, "people", len(r.People), 22)
		for i, w := range []Person{
			{"Krysta Youngs", core.CreditComposer, ""},
			{"Julia Ross", core.CreditComposer, ""},
			{"Yiwoomin", core.CreditComposer, ""},
			{"Ji-hyo Park", core.CreditLyricist, ""},
			{"Yiwoomin", core.CreditActor, "Electric Piano"},
		} {
			check(t, "person", r.People[i], w)
		}
		check(t, "genres", len(md.Genres), 4)
		for _, g := range []string{"Electronic", "Trance", "Dance", "Jazz"} {
			if !slices.Contains(md.Genres, g) {
				t.Errorf("genres: got = %q, want %q", md.Genres, g)
			}
		}
	},
}

func TestNormalize(t *testing.T) {
	for name, check := range probeFacts {
		t.Run(name, check)
	}
}
