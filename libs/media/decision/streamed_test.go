package decision

import (
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

func TestStreamedSubtitles(t *testing.T) {
	web := []SubtitleProfile{{Format: "vtt", Method: SubtitleExternal}, {Format: "vtt", Method: SubtitleHLS}}
	native := append([]SubtitleProfile{{Format: "subrip", Method: SubtitleEmbed}}, web...)
	for _, tt := range []struct {
		name      string
		streamed  bool
		profiles  []SubtitleProfile
		method    Method
		subtitles SubtitleMethod
	}{
		{"web, local", false, web, DirectPlay, SubtitleExternal},
		// The subtitles come from the HLS remux, which reads them as it plays.
		{"web, cloud", true, web, Transcode, SubtitleHLS},
		// The client demuxes them.
		{"native, cloud", true, native, DirectPlay, SubtitleEmbed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{
				MediaSource: &core.MediaSource{
					ID: sourceID("test-source"), Path: "/media/movie.mkv", Container: "mkv", Bitrate: 4_000_000,
					Streams: []core.MediaStream{
						{Kind: core.StreamVideo, Index: 0, Codec: "h264"},
						{Kind: core.StreamAudio, Index: 1, Codec: "aac"},
						{Kind: core.StreamSubtitle, Index: 2, Codec: "subrip", Language: "eng"},
					},
				},
				StreamedSubtitles: tt.streamed,
			}
			client := &ClientCapabilities{
				Name:                "Client",
				MaxStreamingBitrate: 8_000_000, MaxStaticBitrate: 8_000_000,
				DirectPlay:  []DirectPlayProfile{{Kind: Video, Container: "mkv", VideoCodec: "h264", AudioCodec: "aac"}},
				Transcoding: []TranscodingProfile{{Kind: Video, Context: Streaming, Protocol: HLS, Container: "mp4", VideoCodec: "h264", AudioCodec: "aac"}},
				Subtitles:   tt.profiles,
			}
			sub := 2
			d, err := (&Builder{Transcoder: fakeTranscoder{extract: true}}).Video(&Request{
				Sources: []*Source{s}, SourceID: s.ID, Client: client, Context: Streaming, SubtitleStreamIndex: &sub,
				EnableDirectPlay: true, EnableDirectStream: true, AllowAudioStreamCopy: true, AllowVideoStreamCopy: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if d.Method != tt.method || d.SubtitleMethod != tt.subtitles {
				t.Errorf("got = %s with %s subtitles, want = %s with %s", d.Method, d.SubtitleMethod, tt.method, tt.subtitles)
			}
		})
	}
}
