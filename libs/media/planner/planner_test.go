package planner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/decision"
	"github.com/mavioai/mavio/libs/media/hwaccel"
)

func joined(args []string) string { return strings.Join(args, " ") }

func contains(t *testing.T, args []string, want string) {
	t.Helper()
	if !strings.Contains(joined(args), want) {
		t.Errorf("got = %q, want it to contain %q", joined(args), want)
	}
}

func lacks(t *testing.T, args []string, unwanted string) {
	t.Helper()
	if strings.Contains(joined(args), unwanted) {
		t.Errorf("got = %q, want it without %q", joined(args), unwanted)
	}
}

// videoJob ports BuildState: an MKV with H.264 video, AAC audio and the
// given subtitle streams.
func videoJob(sub *core.MediaStream, method decision.SubtitleMethod, more ...core.MediaStream) *Job {
	streams := []core.MediaStream{
		{Index: 0, Kind: core.StreamVideo, Codec: "h264"},
		{Index: 1, Kind: core.StreamAudio, Codec: "aac"},
	}
	switch {
	case more != nil:
		streams = append(streams, more...)
	case sub != nil:
		streams = append(streams, *sub)
	}
	src := &decision.Source{MediaSource: &core.MediaSource{Container: "mkv", Streams: streams}}
	j := &Job{Delivery: Progressive, Source: src, Video: &src.Streams[0], Audio: &src.Streams[1], SubtitleMethod: method, VideoCodec: "h264"}
	if sub != nil {
		for i := range src.Streams {
			if src.Streams[i].Index == sub.Index {
				j.Subtitle = &src.Streams[i]
			}
		}
	}
	if method == "" {
		j.SubtitleMethod = decision.SubtitleDrop
	}
	return j
}

func srt(index int, path string) core.MediaStream {
	return core.MediaStream{Index: index, Kind: core.StreamSubtitle, Codec: "srt", ExternalPath: path}
}

// audioJob ports BuildAudioState: a FLAC track at 96 kHz.
func audioJob(codec string, sampleRate int, container string) *Job {
	src := &decision.Source{MediaSource: &core.MediaSource{
		Container: "flac", Path: "/media/track.flac",
		Streams: []core.MediaStream{{Index: 0, Kind: core.StreamAudio, Codec: "flac", SampleRate: 96000}},
	}}
	return &Job{Delivery: Progressive, Source: src, Audio: &src.Streams[0], AudioCodec: codec, AudioSampleRate: sampleRate, Container: container}
}

func TestEncodingHelperCases(t *testing.T) {
	p := &Planner{Options: DefaultOptions()}
	methods := map[string]decision.SubtitleMethod{"Embed": decision.SubtitleEmbed, "Encode": decision.SubtitleEncode}
	portedCases(t, "encoding_helper.json", ported{
		run: map[string]func(t *testing.T, a args){
			"GetInputArgument_VobSub_UsesCorrectPath": func(t *testing.T, a args) {
				dir := t.TempDir()
				sub := filepath.Join(dir, "movie.sub")
				if err := os.WriteFile(sub, []byte("dummy"), 0o644); err != nil {
					t.Fatal(err)
				}
				if a.boolean(t, "createIdxFile") {
					if err := os.WriteFile(filepath.Join(dir, "movie.idx"), []byte("dummy"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				st := core.MediaStream{Index: 2, Kind: core.StreamSubtitle, Codec: "dvdsub", ExternalPath: sub}
				j := videoJob(&st, methods[a.symbol(t, "deliveryMethod")])
				fs := &Planner{Options: DefaultOptions(), FileExists: func(path string) bool { _, err := os.Stat(path); return err == nil }}
				contains(t, fs.inputArgs(j), a.str(t, "expectedFilename"))
			},
			"GetProgressiveAudioFullCommandLine_SampleRate_OnlyClampedForOpus": func(t *testing.T, a args) {
				j := audioJob(a.str(t, "audioCodec"), a.integer(t, "requestedSampleRate"), "")
				contains(t, p.ProgressiveAudioArgs(j, "/tmp/out"), "-ar "+itoa(a.integer(t, "expectedSampleRate")))
			},
			"GetProgressiveAudioFullCommandLine_PcmInRealContainer_KeepsContainerMuxer": func(t *testing.T, a args) {
				lacks(t, p.ProgressiveAudioArgs(audioJob("pcm_s16le", 48000, a.str(t, "outputContainer")), "/tmp/out"), "-f s16le")
			},
			"GetProgressiveVideoAudioArguments_NonStereoOutput_KeepsChannelCount": func(t *testing.T, a args) {
				j := audioJob("aac", 48000, "")
				j.Audio.Channels, j.Audio.ChannelLayout = 6, "5.1"
				n := a.integer(t, "outputChannels")
				j.AudioChannels = n
				q := &Planner{Options: DefaultOptions()}
				q.Options.Downmix = DownmixDave750
				args := q.audioArgs(j, q.AudioEncoder(j))
				contains(t, args, "-ac "+itoa(n))
				lacks(t, args, "pan=")
			},
		},
		facts: map[string]string{
			"GetMapArgs_NoSubtitle_ExcludesAllSubs":                                           "TestMapArgs",
			"GetMapArgs_InternalSrt_MapsFromPrimaryInput":                                     "TestMapArgs",
			"GetMapArgs_InternalSubAtHigherIndex_MapsCorrectIndex":                            "TestMapArgs",
			"GetMapArgs_ExternalSrt_MapsFirstStreamFromInput1":                                "TestMapArgs",
			"GetMapArgs_SecondExternalSrt_StillMaps1Colon0":                                   "TestMapArgs",
			"GetMapArgs_MksFirstTrack_MapsInFileIndex0":                                       "TestMapArgs",
			"GetMapArgs_MksSecondTrack_MapsInFileIndex1":                                      "TestMapArgs",
			"GetProgressiveAudioFullCommandLine_PcmInPcmContainer_ForcesRawMuxer":             "TestProgressiveAudio",
			"GetProgressiveAudioFullCommandLine_PcmWithoutBitrate_EmitsNoEmptySampleRate":     "TestProgressiveAudio",
			"GetProgressiveAudioFullCommandLine_StereoDownmix_AppliesDownMixAlgorithm":        "TestProgressiveAudio",
			"GetProgressiveAudioFullCommandLine_NoDownmix_EmitsNoAudioFilter":                 "TestProgressiveAudio",
			"GetProgressiveVideoAudioArguments_StereoDownmix_UsesFilterInsteadOfChannelCount": "TestProgressiveAudio",
		},
	})
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestMapArgs(t *testing.T) {
	p := &Planner{Options: DefaultOptions()}
	t.Run("no subtitle", func(t *testing.T) {
		args := p.MapArgs(videoJob(nil, ""), true)
		contains(t, args, "-map -0:s")
		lacks(t, args, "-map 1:")
	})
	t.Run("internal srt", func(t *testing.T) {
		s := srt(2, "")
		args := p.MapArgs(videoJob(&s, decision.SubtitleEmbed), true)
		contains(t, args, "-map 0:2")
		lacks(t, args, "-map 1:")
	})
	t.Run("internal at higher index", func(t *testing.T) {
		s0, s1 := srt(2, ""), core.MediaStream{Index: 3, Kind: core.StreamSubtitle, Codec: "ass"}
		contains(t, p.MapArgs(videoJob(&s1, decision.SubtitleEmbed, s0, s1), true), "-map 0:3")
	})
	t.Run("external srt", func(t *testing.T) {
		s := srt(2, "/media/movie.en.srt")
		contains(t, p.MapArgs(videoJob(&s, decision.SubtitleEmbed), true), "-map 1:0")
	})
	t.Run("second external srt", func(t *testing.T) {
		e1, e2 := srt(2, "/media/movie.en.srt"), srt(3, "/media/movie.fr.srt")
		contains(t, p.MapArgs(videoJob(&e2, decision.SubtitleEmbed, e1, e2), true), "-map 1:0")
	})
	mks := func(i int, codec string) core.MediaStream {
		return core.MediaStream{Index: i, Kind: core.StreamSubtitle, Codec: codec, ExternalPath: "/media/movie.mks"}
	}
	t.Run("mks first track", func(t *testing.T) {
		m0, m1 := mks(2, "subrip"), mks(3, "ass")
		contains(t, p.MapArgs(videoJob(&m0, decision.SubtitleEmbed, m0, m1), true), "-map 1:0")
	})
	t.Run("mks second track", func(t *testing.T) {
		m0, m1, m2 := mks(2, "subrip"), mks(3, "ass"), mks(4, "subrip")
		contains(t, p.MapArgs(videoJob(&m1, decision.SubtitleEmbed, m0, m1, m2), true), "-map 1:1")
	})
}

func TestProgressiveAudio(t *testing.T) {
	p := &Planner{Options: DefaultOptions()}
	t.Run("pcm container forces raw muxer", func(t *testing.T) {
		contains(t, p.ProgressiveAudioArgs(audioJob("pcm_s16le", 48000, "pcm"), "/tmp/out"), "-f s16le")
	})
	t.Run("pcm without bitrate", func(t *testing.T) {
		j := audioJob("pcm_s16le", 48000, "wav")
		args := p.ProgressiveAudioArgs(j, "/tmp/out")
		contains(t, args, "-ar 48000")
		lacks(t, args, "-ar -")
		lacks(t, args, "-ab")
	})
	downmix := func(boost float64) *Planner {
		q := &Planner{Options: DefaultOptions()}
		q.Options.Downmix, q.Options.DownmixBoost = DownmixDave750, boost
		return q
	}
	surround := func() *Job {
		j := audioJob("aac", 48000, "")
		j.Audio.Channels, j.Audio.ChannelLayout = 6, "5.1"
		j.AudioChannels = 2
		return j
	}
	t.Run("stereo downmix applies the algorithm", func(t *testing.T) {
		f, _ := DownmixFilter(DownmixDave750, "5.1")
		contains(t, downmix(1).ProgressiveAudioArgs(surround(), "/tmp/out"), "-af "+f)
	})
	t.Run("no downmix", func(t *testing.T) {
		j := audioJob("aac", 48000, "")
		j.Audio.Channels, j.AudioChannels = 2, 2
		lacks(t, downmix(2).ProgressiveAudioArgs(j, "/tmp/out"), "-af")
	})
	t.Run("video audio stereo downmix uses the filter", func(t *testing.T) {
		q := downmix(2)
		j := surround()
		args := q.audioArgs(j, q.AudioEncoder(j))
		lacks(t, args, "-ac ")
		contains(t, args, "pan=stereo")
	})
}

func TestAudioBitStreamCases(t *testing.T) {
	const file, class = "audio_bit_stream.json", "EncodingHelperAudioBitStreamTests"
	portedCases(t, "encoding_helper_audio_bit_stream.json", ported{
		run: map[string]func(t *testing.T, a args){
			"AudioBitStreamArguments_AppliesGates": func(t *testing.T, a args) {
				var start int64
				var version, expected string
				for name, v := range map[string]any{"startTicks": &start, "ffmpegVersion": &version, "expected": &expected} {
					if err := json.Unmarshal(a.constant(t, name, file, class), v); err != nil {
						t.Fatalf("%s: %v", name, err)
					}
				}
				v, ok := hwaccel.ParseVersionString(version)
				if !ok {
					v, _ = hwaccel.ParseVersionString(version + ".0")
				}
				delivery := Progressive
				if a.symbol(t, "jobType") == "Hls" {
					delivery = HLS
				}
				j := &Job{
					Delivery:   delivery,
					VideoCodec: a.str(t, "outputVideoCodec"),
					AudioCodec: a.str(t, "outputAudioCodec"),
					Audio:      &core.MediaStream{Kind: core.StreamAudio, Codec: a.str(t, "audioStreamCodec")},
					Source:     &decision.Source{MediaSource: &core.MediaSource{Container: a.str(t, "inputContainer")}},
					Start:      time.Duration(start) * 100,
				}
				p := &Planner{Options: DefaultOptions(), Caps: &hwaccel.Capabilities{Version: v}}
				if got := p.AudioBitstreamArgs(j, a.str(t, "segmentContainer"), a.str(t, "mediaSourceContainer")); got != expected {
					t.Errorf("got = %q, want = %q", got, expected)
				}
			},
		},
	})
}

func TestInferAudioCodecCases(t *testing.T) {
	portedCases(t, "encoding_helper_infer_audio_codec.json", ported{
		run: map[string]func(t *testing.T, a args){
			"InferAudioCodec_ReturnsAnAudioCodec": func(t *testing.T, a args) {
				if got, want := InferAudioCodec(a.str(t, "container")), a.str(t, "expected"); got != want {
					t.Errorf("got = %s, want = %s", got, want)
				}
			},
		},
	})
}

func TestTranscodingJobCases(t *testing.T) {
	const reason = "segment bookkeeping is ported in libs/media/supervisor (TestServedSegments)"
	portedCases(t, "transcoding_job.json", ported{
		skip: map[string]string{
			"GetHighestServedSegmentIndexEndingAtOrBefore_NoSegmentsServed_ReturnsNull":                 reason,
			"GetHighestServedSegmentIndexEndingAtOrBefore_InitSegment_IsIgnored":                        reason,
			"GetHighestServedSegmentIndexEndingAtOrBefore_SegmentEndingExactlyAtPosition_IsIncluded":    reason,
			"GetHighestServedSegmentIndexEndingAtOrBefore_SegmentsLongerThanDesired_ReturnsServedIndex": reason,
		},
	})
}
