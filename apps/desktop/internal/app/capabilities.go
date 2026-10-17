package app

import (
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
)

// capabilities returns what the app plays with mpv, which demuxes with
// FFmpeg and decodes what the hardware cannot in software: the server
// plays nearly every source as it is, and processes into HLS only what
// exceeds maxBitrate, when not zero.
func capabilities(maxBitrate int64) *playbackv1.ClientCapabilities {
	video, audio := playbackv1.MediaKind_MEDIA_KIND_VIDEO, playbackv1.MediaKind_MEDIA_KIND_AUDIO
	direct := func(kind playbackv1.MediaKind, container, videoCodec, audioCodec string) *playbackv1.DirectPlayProfile {
		return playbackv1.DirectPlayProfile_builder{Kind: &kind, Container: &container, VideoCodec: &videoCodec, AudioCodec: &audioCodec}.Build()
	}
	hls := func(kind playbackv1.MediaKind, videoCodec, audioCodec string, channels int32) *playbackv1.TranscodingProfile {
		return playbackv1.TranscodingProfile_builder{
			Kind: &kind, Protocol: new(playbackv1.Protocol_PROTOCOL_HLS), Container: new("mp4"),
			VideoCodec: &videoCodec, AudioCodec: &audioCodec, MaxAudioChannels: &channels,
			EnableSubtitlesInManifest: new(true),
		}.Build()
	}
	var subtitles []*playbackv1.SubtitleProfile
	subtitle := func(method playbackv1.SubtitleMethod, formats ...string) {
		for _, f := range formats {
			subtitles = append(subtitles, playbackv1.SubtitleProfile_builder{Format: &f, Method: &method}.Build())
		}
	}
	// mpv draws what the file carries, and files of their own.
	subtitle(playbackv1.SubtitleMethod_SUBTITLE_METHOD_EMBED,
		"ass", "ssa", "subrip", "srt", "vtt", "webvtt", "mov_text", "pgssub", "dvdsub", "dvbsub")
	subtitle(playbackv1.SubtitleMethod_SUBTITLE_METHOD_EXTERNAL, "srt", "subrip", "vtt", "ass", "ssa")
	subtitle(playbackv1.SubtitleMethod_SUBTITLE_METHOD_HLS, "vtt")
	return playbackv1.ClientCapabilities_builder{
		Name:                new("Mavio Desktop"),
		MaxStreamingBitrate: &maxBitrate,
		DirectPlay: []*playbackv1.DirectPlayProfile{
			direct(video, "mkv,webm,mp4,m4v,mov,avi,ts,m2ts,mpegts,flv,wmv,asf,ogv",
				"h264,hevc,av1,vp9,vp8,mpeg2video,mpeg4,vc1",
				"aac,mp3,mp2,ac3,eac3,dts,truehd,flac,alac,opus,vorbis,pcm_s16le,pcm_s24le"),
			direct(audio, "mp3,m4a,mp4,aac,flac,wav,aiff,alac,ogg,oga,opus,webm,mka,ape,wv,dsf", "", ""),
		},
		Transcoding: []*playbackv1.TranscodingProfile{
			hls(video, "hevc,h264", "aac,ac3,eac3", 6),
			hls(audio, "", "aac", 2),
		},
		Subtitles: subtitles,
	}.Build()
}
