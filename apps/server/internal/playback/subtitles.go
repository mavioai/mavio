package playback

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/language"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/decision"
	"github.com/mavioai/mavio/libs/streaming"
	"github.com/mavioai/mavio/libs/subtitle"
)

// errNoSubtitle is returned for subtitle streams that cannot be served
// as text.
var errNoSubtitle = errors.New("subtitle cannot be served as text")

// subtitleGroup names the subtitle renditions of HLS master playlists.
const subtitleGroup = "subs"

// SubtitleURL returns the path of a subtitle file delivered on its own,
// relative to the server's base URL; "" for subtitles delivered otherwise
// or that cannot be served as text.
func (p *Playback) SubtitleURL(s *Subtitle) string {
	if s.Method != decision.SubtitleExternal || !servable(s) {
		return ""
	}
	return "media/" + p.ID + "/subtitles/" + strconv.Itoa(s.Stream.Index) + "." + strings.ToLower(s.Format)
}

// servable reports whether a subtitle stream can be read and written in
// its delivery format.
func servable(s *Subtitle) bool {
	return s.Stream.IsTextSubtitle() && subtitle.CanWrite(s.Format) && sourceFormat(s.Stream) != ""
}

// hlsSubtitles lists the subtitle renditions of an HLS playback.
func (p *Playback) hlsSubtitles() []streaming.Subtitle {
	var out []streaming.Subtitle
	for i := range p.Subtitles {
		s := &p.Subtitles[i]
		if s.Method != decision.SubtitleHLS || !s.Stream.IsTextSubtitle() || sourceFormat(s.Stream) == "" {
			continue
		}
		name := s.Stream.Title
		if name == "" {
			name = s.Stream.Language
		}
		if name == "" {
			name = "Subtitles " + strconv.Itoa(s.Stream.Index)
		}
		out = append(out, streaming.Subtitle{
			Group: subtitleGroup, Name: name, Language: bcp47(s.Stream.Language),
			URI:     "subtitles/" + strconv.Itoa(s.Stream.Index) + ".m3u8",
			Default: s.Stream.Index == p.SubtitleStream, Forced: s.Stream.Forced,
		})
	}
	return out
}

// bcp47 converts an ISO 639-2 code to the BCP 47 tag HLS expects, "" when
// unknown.
func bcp47(code string) string {
	tag, err := language.Parse(code)
	if err != nil || tag == language.Und {
		return ""
	}
	return tag.String()
}

// sourceFormat is the format subtitle text is read in: that of an external
// file, or the one ffmpeg extracts embedded text to.
func sourceFormat(st *core.MediaStream) string {
	codec := strings.ToLower(st.Codec)
	switch {
	case codec == "ass" || codec == "ssa":
		return subtitle.ASS
	case st.ExternalPath != "" && subtitle.CanParse(codec):
		return codec
	case st.ExternalPath != "":
		return ""
	}
	return subtitle.SRT
}

// serveSubtitle serves "{index}.{format}" or, for HLS renditions,
// "{index}.m3u8".
func (m *Manager) serveSubtitle(w http.ResponseWriter, r *http.Request, p *Playback, file string) {
	name, ext, _ := strings.Cut(file, ".")
	index, err := strconv.Atoi(name)
	var sub *Subtitle
	for i := range p.Subtitles {
		if s := &p.Subtitles[i]; err == nil && s.Stream.Index == index && strconv.Itoa(index) == name {
			sub = s
		}
	}
	if sub == nil {
		http.NotFound(w, r)
		return
	}
	if ext == "m3u8" && sub.Method == decision.SubtitleHLS && p.HLS() {
		// One WebVTT segment spanning the whole source.
		w.Header().Set("Content-Type", playlistType)
		_, _ = streaming.MediaPlaylist{
			Segments:  []time.Duration{max(p.Source().Duration, time.Second)},
			Container: "vtt",
			URI:       func(int) string { return name + ".vtt" },
		}.WriteTo(w)
		return
	}
	format := strings.ToLower(sub.Format)
	if sub.Method == decision.SubtitleHLS {
		format = subtitle.VTT
	}
	if ext != format || !subtitle.CanWrite(format) {
		http.NotFound(w, r)
		return
	}
	s, err := m.loadSubtitle(r.Context(), p, sub.Stream)
	if errors.Is(err, errNoSubtitle) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		if !errors.Is(err, context.Canceled) {
			m.log.ErrorContext(r.Context(), "read subtitle", "playback", p.ID, "stream", index, "err", err)
			http.Error(w, "subtitle unavailable", http.StatusInternalServerError)
		}
		return
	}
	data, err := subtitle.Write(s, format)
	if err != nil {
		m.log.ErrorContext(r.Context(), "write subtitle", "playback", p.ID, "stream", index, "err", err)
		http.Error(w, "subtitle unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", subtitleTypes[format])
	_, _ = w.Write(data)
}

var subtitleTypes = map[string]string{
	subtitle.VTT: "text/vtt; charset=utf-8", subtitle.SRT: "application/x-subrip; charset=utf-8",
	subtitle.ASS: "text/x-ssa; charset=utf-8", subtitle.SSA: "text/x-ssa; charset=utf-8",
	subtitle.TTML: "application/ttml+xml; charset=utf-8", subtitle.JSON: "application/json",
}

// loadSubtitle reads a text subtitle stream once per playback: an external
// file from the library, or an embedded stream extracted with ffmpeg.
func (m *Manager) loadSubtitle(ctx context.Context, p *Playback, st *core.MediaStream) (*subtitle.Subtitle, error) {
	format := sourceFormat(st)
	if !st.IsTextSubtitle() || format == "" {
		return nil, errNoSubtitle
	}
	p.subsMu.Lock()
	defer p.subsMu.Unlock()
	if s, ok := p.subs[st.Index]; ok {
		return s, nil
	}
	var data []byte
	if st.ExternalPath != "" {
		f, _, err := openInRoot(p.root, st.ExternalPath)
		if err != nil {
			return nil, err
		}
		data, err = io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			return nil, err
		}
	} else {
		if m.cfg.FFmpegPath == "" {
			return nil, errNoSubtitle
		}
		var err error
		if data, err = extractSubtitle(ctx, m.cfg.FFmpegPath, p.Source().Path, st.Index, format); err != nil {
			return nil, err
		}
	}
	data, _, err := subtitle.ToUTF8(data)
	if err != nil {
		return nil, err
	}
	s, err := subtitle.Parse(data, format)
	if err != nil {
		return nil, err
	}
	if p.subs == nil {
		p.subs = map[int]*subtitle.Subtitle{}
	}
	p.subs[st.Index] = s
	return s, nil
}

// extractSubtitle copies an embedded text subtitle stream out of a file,
// converted to format: ASS keeps its styling, other text becomes SRT.
// Cues are timed from the source's start, as HLS outputs are.
func extractSubtitle(ctx context.Context, ffmpeg, path string, index int, format string) ([]byte, error) {
	codec := "srt"
	if format == subtitle.ASS {
		codec = "ass"
	}
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostdin", "-loglevel", "error",
		"-i", path, "-map", "0:"+strconv.Itoa(index), "-c:s", codec, "-f", codec, "pipe:1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("extract subtitle %d: %w: %s", index, err, bytes.TrimSpace(stderr.Bytes()))
	}
	return out, nil
}
