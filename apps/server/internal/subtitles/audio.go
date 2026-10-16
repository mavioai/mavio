package subtitles

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/media/vad"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// AudioPath is the pattern of the route serving videos' audio to plugins
// through the host API: to recognizers transcribing subtitles, and to
// subtitle provider plugins.
const AudioPath = "GET /subtitles/audio/{token}"

// audioBase is the URL of the audio route in the host API.
const audioBase = "http://mavio.host/subtitles/audio/"

// audioTTL is how long an audio URL serves after a search or download
// last gave it.
const audioTTL = 24 * time.Hour

// audioGrant is a video whose audio a URL serves.
type audioGrant struct {
	lib     core.Library
	path    string
	expires time.Time
}

// audioURL returns the URL serving the audio of the target's video to
// provider plugins, or "" when the server does not read it: on cloud
// drives, whose reads are counted, or without ffmpeg. The URL's token is
// unguessable, and kept while searches and downloads give it.
func (r *Runtime) audioURL(ctx context.Context, t *target) string {
	src := t.source()
	if r.FFmpeg == "" || !r.readable(ctx, src.Path) {
		return ""
	}
	now := r.now()
	r.audioMu.Lock()
	defer r.audioMu.Unlock()
	if r.audioGrants == nil {
		r.audioGrants, r.audioTokens = map[string]*audioGrant{}, map[string]string{}
	}
	for token, g := range r.audioGrants {
		if now.After(g.expires) {
			delete(r.audioGrants, token)
			delete(r.audioTokens, g.path)
		}
	}
	token, ok := r.audioTokens[src.Path]
	if !ok {
		token = rand.Text()
		r.audioTokens[src.Path] = token
	}
	r.audioGrants[token] = &audioGrant{lib: t.lib, path: src.Path, expires: now.Add(audioTTL)}
	return audioBase + token
}

// audioStreams returns a video's audio streams as provider plugins take
// them.
func audioStreams(streams []core.MediaStream) []*pluginv1.SubtitleAudioStream {
	var out []*pluginv1.SubtitleAudioStream
	for _, st := range streams {
		if st.Kind != core.StreamAudio || st.ExternalPath != "" {
			continue
		}
		s := &pluginv1.SubtitleAudioStream{}
		s.SetIndex(int32(len(out)))
		s.SetLanguage(st.Language)
		out = append(out, s)
	}
	return out
}

// AudioHandler serves AudioPath: an audio stream of the video a token
// grants, as 16 kHz mono 16-bit little-endian PCM, padded or trimmed at
// its start to begin with the video, as Bazarr's Whisper provider
// encodes it. The query parameter "stream" picks the audio stream, from
// 0; "seconds" serves only the first seconds.
func (r *Runtime) AudioHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.audioMu.Lock()
		g, ok := r.audioGrants[req.PathValue("token")]
		r.audioMu.Unlock()
		if !ok || r.now().After(g.expires) {
			http.NotFound(w, req)
			return
		}
		q := req.URL.Query()
		stream, seconds := 0, 0
		for name, v := range map[string]*int{"stream": &stream, "seconds": &seconds} {
			if s := q.Get(name); s != "" {
				n, err := strconv.Atoi(s)
				if err != nil || n < 0 {
					http.Error(w, "invalid "+name, http.StatusBadRequest)
					return
				}
				*v = n
			}
		}
		if err := r.serveAudio(req.Context(), w, g, stream, seconds); err != nil {
			r.logger().WarnContext(req.Context(), "serve audio", "path", g.path, "err", err)
		}
	})
}

// headerWriter writes the response's header before its first bytes, so
// that failures before them are answered with an error.
type headerWriter struct {
	w       http.ResponseWriter
	started bool
}

func (h *headerWriter) Write(p []byte) (int, error) {
	if !h.started {
		h.started = true
		h.w.Header().Set("Content-Type", "application/octet-stream")
		h.w.WriteHeader(http.StatusOK)
	}
	return h.w.Write(p)
}

func (r *Runtime) serveAudio(ctx context.Context, w http.ResponseWriter, g *audioGrant, stream, seconds int) error {
	root, name, ok := library.RootOf(g.lib.Paths, g.path)
	if !ok {
		http.Error(w, "video outside its library", http.StatusNotFound)
		return fmt.Errorf("%s is outside the library's folders", g.path)
	}
	st, err := r.Storages.Open(ctx, root)
	if err != nil {
		http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
		return err
	}
	defer st.Close()
	input, err := st.MediaURL(core.WithReadIntent(ctx, core.ReadIntent{Kind: core.ReadBackground}), name)
	if err != nil {
		http.Error(w, "video unavailable", http.StatusServiceUnavailable)
		return err
	}
	cmd := exec.CommandContext(ctx, r.FFmpeg, audioArgs(input, stream, seconds)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	hw := &headerWriter{w: w}
	cmd.Stdout = hw
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		if !hw.started {
			http.Error(w, "audio unavailable", http.StatusUnprocessableEntity)
		}
		return fmt.Errorf("ffmpeg: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	if !hw.started {
		http.Error(w, "no audio", http.StatusUnprocessableEntity)
		return errors.New("no audio")
	}
	return nil
}

// audioArgs returns the ffmpeg arguments writing the stream'th audio
// stream of input, or its first seconds when seconds is positive, as
// 16 kHz mono 16-bit samples starting with the video.
func audioArgs(input string, stream, seconds int) []string {
	args := []string{
		"-v", "error", "-nostdin", "-i", input, "-map", "0:a:" + strconv.Itoa(stream), "-vn", "-sn", "-dn",
		"-ac", "1", "-ar", strconv.Itoa(vad.SampleRate), "-af", "aresample=async=1:first_pts=0",
	}
	if seconds > 0 {
		args = append(args, "-t", strconv.Itoa(seconds))
	}
	return append(args, "-c:a", "pcm_s16le", "-f", "s16le", "-")
}
