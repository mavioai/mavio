package fixtures

// Spec describes one generated test media file.
type Spec struct {
	// Name is the output file name inside the fixtures directory.
	Name string
	// Description says what the fixture exercises.
	Description string
	// Encoders lists the ffmpeg encoders the fixture needs. The fixture is
	// skipped when the local ffmpeg build lacks any of them.
	Encoders []string
	// Files are auxiliary input files (subtitles, chapter metadata, …) keyed by
	// file name. Args reference them as "{file:<name>}".
	Files map[string]string
	// Args are the ffmpeg arguments between the global options and the output
	// path.
	Args []string
}

const (
	srtEnglish = `1
00:00:00,000 --> 00:00:01,000
Hello from Mavio.

2
00:00:01,000 --> 00:00:02,000
Second line.
`
	srtJapanese = `1
00:00:00,000 --> 00:00:01,000
こんにちは。

2
00:00:01,000 --> 00:00:02,000
二行目。
`
	ffmetadataChapters = `;FFMETADATA1
title=Chapter Test

[CHAPTER]
TIMEBASE=1/1000
START=0
END=1000
title=Opening

[CHAPTER]
TIMEBASE=1/1000
START=1000
END=2000
title=Ending
`
)

// lavfi inputs shared by most fixtures.
func video(size, rate, duration string) []string {
	return []string{"-f", "lavfi", "-i", "testsrc2=size=" + size + ":rate=" + rate + ":duration=" + duration}
}

func tone(freq, duration string) []string {
	return []string{"-f", "lavfi", "-i", "sine=frequency=" + freq + ":sample_rate=48000:duration=" + duration}
}

func concat(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// Catalog returns every fixture in a stable order.
func Catalog() []Spec {
	return []Spec{
		{
			Name:        "h264_aac.mp4",
			Description: "Baseline web-compatible file: H.264 + stereo AAC in MP4.",
			Encoders:    []string{"libx264", "aac"},
			Args: concat(video("640x360", "24", "2"), tone("440", "2"),
				[]string{"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-c:a", "aac", "-ac", "2", "-b:a", "128k"}),
		},
		{
			Name:        "hevc_main10_hdr10.mkv",
			Description: "HDR10: HEVC Main10 with BT.2020 / PQ signaling, mastering display and content light level metadata, plus 5.1 AC-3.",
			Encoders:    []string{"libx265", "ac3"},
			Args: concat(video("1280x720", "24", "2"), tone("440", "2"),
				[]string{
					// Tag the frames so the container-level color description matches the bitstream.
					"-vf", "setparams=color_primaries=bt2020:color_trc=smpte2084:colorspace=bt2020nc,format=yuv420p10le",
					"-c:v", "libx265", "-preset", "ultrafast",
					"-x265-params", "log-level=error:pools=1:frame-threads=1:hdr10=1:repeat-headers=1:colorprim=bt2020:transfer=smpte2084:colormatrix=bt2020nc:" +
						"master-display=G(13250,34500)B(7500,3000)R(34000,16000)WP(15635,16450)L(10000000,1):max-cll=1000,400",
					"-c:a", "ac3", "-ac", "6",
				}),
		},
		{
			Name:        "multi_track.mkv",
			Description: "Track selection: two audio tracks (eng AAC stereo, jpn AC-3 5.1) and two subtitle tracks (eng SRT, jpn ASS) with languages, titles and dispositions.",
			Encoders:    []string{"libx264", "aac", "ac3", "srt", "ass"},
			Files:       map[string]string{"eng.srt": srtEnglish, "jpn.srt": srtJapanese},
			Args: concat(video("640x360", "24", "2"), tone("440", "2"), tone("660", "2"),
				[]string{"-i", "{file:eng.srt}", "-i", "{file:jpn.srt}"},
				[]string{
					"-map", "0:v", "-map", "1:a", "-map", "2:a", "-map", "3:s", "-map", "4:s",
					"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
					"-c:a:0", "aac", "-ac:a:0", "2", "-c:a:1", "ac3", "-ac:a:1", "6",
					"-c:s:0", "srt", "-c:s:1", "ass",
					"-metadata:s:a:0", "language=eng", "-metadata:s:a:0", "title=English Stereo",
					"-metadata:s:a:1", "language=jpn", "-metadata:s:a:1", "title=Japanese 5.1",
					"-metadata:s:s:0", "language=eng", "-metadata:s:s:1", "language=jpn",
					"-disposition:a:0", "default", "-disposition:a:1", "0",
					"-disposition:s:0", "0", "-disposition:s:1", "forced",
				}),
		},
		{
			Name:        "mpeg2_interlaced.ts",
			Description: "Broadcast-style interlaced MPEG-2 (top field first) with MP2 audio in MPEG-TS.",
			Encoders:    []string{"mpeg2video", "mp2"},
			Args: concat(video("720x480", "30000/1001", "2"), tone("440", "2"),
				[]string{
					// Mark the frames interlaced, top field first, before interlaced encoding.
					"-vf", "setfield=tff",
					"-c:v", "mpeg2video", "-b:v", "4M", "-flags", "+ilme+ildct", "-top", "1", "-pix_fmt", "yuv420p",
					"-c:a", "mp2", "-b:a", "192k",
				}),
		},
		{
			Name:        "vp9_opus.webm",
			Description: "WebM with VP9 video and Opus audio.",
			Encoders:    []string{"libvpx-vp9", "libopus"},
			Args: concat(video("640x360", "24", "2"), tone("440", "2"),
				[]string{"-c:v", "libvpx-vp9", "-deadline", "realtime", "-cpu-used", "8", "-b:v", "500k", "-c:a", "libopus", "-b:a", "96k"}),
		},
		{
			Name:        "av1_opus.mkv",
			Description: "AV1 (SVT-AV1) video with Opus audio in Matroska.",
			Encoders:    []string{"libsvtav1", "libopus"},
			Args: concat(video("640x360", "24", "2"), tone("440", "2"),
				[]string{"-c:v", "libsvtav1", "-preset", "12", "-crf", "40", "-svtav1-params", "lp=1", "-c:a", "libopus", "-b:a", "96k"}),
		},
		{
			Name:        "ntsc_film_odd_duration.mp4",
			Description: "23.976 fps (24000/1001) with a 1.5 s duration, for frame-rate and duration rounding.",
			Encoders:    []string{"libx264", "aac"},
			Args: concat(video("640x360", "24000/1001", "1.5"), tone("440", "1.5"),
				[]string{"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-c:a", "aac"}),
		},
		{
			Name:        "chapters.mkv",
			Description: "Matroska with two named chapters and a container title.",
			Encoders:    []string{"libx264", "aac"},
			Files:       map[string]string{"chapters.txt": ffmetadataChapters},
			Args: concat(video("640x360", "24", "2"), tone("440", "2"),
				[]string{"-i", "{file:chapters.txt}", "-map", "0:v", "-map", "1:a", "-map_metadata", "2", "-map_chapters", "2"},
				[]string{"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-c:a", "aac"}),
		},
		{
			Name:        "tagged.flac",
			Description: "Audio-only FLAC with music tags (title, artist, album, album artist, track, disc, date, genre).",
			Encoders:    []string{"flac"},
			Args: concat(tone("440", "2"),
				[]string{
					"-c:a", "flac",
					"-metadata", "title=Test Tone", "-metadata", "artist=Mavio", "-metadata", "album=Fixtures",
					"-metadata", "album_artist=Mavio", "-metadata", "track=1/2", "-metadata", "disc=1/1",
					"-metadata", "date=2026", "-metadata", "genre=Test",
				}),
		},
	}
}
