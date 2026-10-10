package playback

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/streaming"
)

// mediaTypes are the content types of direct played files by extension;
// the system's MIME tables lack several of them.
var mediaTypes = map[string]string{
	".mp4": "video/mp4", ".m4v": "video/mp4", ".mov": "video/quicktime", ".mkv": "video/x-matroska",
	".webm": "video/webm", ".ts": "video/mp2t", ".m2ts": "video/mp2t", ".avi": "video/x-msvideo",
	".mp3": "audio/mpeg", ".m4a": "audio/mp4", ".m4b": "audio/mp4", ".aac": "audio/aac", ".flac": "audio/flac",
	".ogg": "audio/ogg", ".opus": "audio/ogg", ".wav": "audio/wav", ".mka": "audio/x-matroska",
}

const playlistType = "application/vnd.apple.mpegurl"

// URL returns the path of the playback's media relative to the server's
// base URL: the file for direct play, else the HLS master playlist.
func (p *Playback) URL() string {
	if p.HLS() {
		return "media/" + p.ID + "/master.m3u8"
	}
	return "media/" + p.ID + "/" + directName(p)
}

func directName(p *Playback) string {
	if p.progressive != nil {
		return "stream." + p.progExt
	}
	return "stream" + strings.ToLower(filepath.Ext(p.Source().Path))
}

// Handler serves the media of playbacks at /media/{playback}/{file}. The
// playback ID is the only credential.
func (m *Manager) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /media/{playback}/{file}", m.serveMedia)
	mux.HandleFunc("GET /media/{playback}/attachments/{index}", func(w http.ResponseWriter, r *http.Request) {
		p := m.Get(r.PathValue("playback"))
		if p == nil {
			http.NotFound(w, r)
			return
		}
		p.touch(time.Now())
		m.serveAttachment(w, r, p)
	})
	mux.HandleFunc("GET /media/{playback}/subtitles/{file}", func(w http.ResponseWriter, r *http.Request) {
		p := m.Get(r.PathValue("playback"))
		if p == nil {
			http.NotFound(w, r)
			return
		}
		p.touch(time.Now())
		m.serveSubtitle(w, r, p, r.PathValue("file"))
	})
	return mux
}

func (m *Manager) serveMedia(w http.ResponseWriter, r *http.Request) {
	p := m.Get(r.PathValue("playback"))
	if p == nil {
		http.NotFound(w, r)
		return
	}
	p.touch(time.Now())
	// Every media request, a seek included, keeps background I/O quiet.
	if m.cfg.QuietGate != nil {
		m.cfg.QuietGate.NoteActivity()
	}
	file := r.PathValue("file")
	switch {
	case p.progressive != nil && file == directName(p):
		m.serveProgressive(w, r, p)
	case !p.HLS() && file == directName(p):
		if p.Download {
			w.Header().Set("Content-Disposition", attachment(p, filepath.Ext(p.Source().Path)))
		}
		m.serveFile(w, r, p)
	case p.HLS() && file == "master.m3u8":
		w.Header().Set("Content-Type", playlistType)
		v := p.variant
		v.URI = "main.m3u8"
		subs := p.hlsSubtitles()
		if len(subs) > 0 {
			v.Subtitles = subtitleGroup
		}
		_, _ = streaming.MasterPlaylist{Variants: []streaming.Variant{v}, Subtitles: subs}.WriteTo(w)
	case p.HLS() && file == "main.m3u8":
		w.Header().Set("Content-Type", playlistType)
		_, _ = streaming.MediaPlaylist{
			Segments: p.layout.Segments, Container: p.segExt, URI: p.segmentName,
		}.WriteTo(w)
	case p.HLS():
		index, ok := p.segmentIndex(file)
		if !ok {
			http.NotFound(w, r)
			return
		}
		m.serveSegment(w, r, p, index)
	default:
		http.NotFound(w, r)
	}
}

// segmentName names segment index, or the initialization segment.
func (p *Playback) segmentName(index int) string {
	if index == streaming.InitSegment {
		return "init.mp4"
	}
	return strconv.Itoa(index) + "." + p.segExt
}

func (p *Playback) segmentIndex(file string) (int, bool) {
	if file == "init.mp4" && p.segExt == "mp4" {
		return streaming.InitSegment, true
	}
	name, ok := strings.CutSuffix(file, "."+p.segExt)
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(name)
	if err != nil || i < 0 || i >= len(p.layout.Segments) || strconv.Itoa(i) != name {
		return 0, false
	}
	return i, true
}

func (m *Manager) serveSegment(w http.ResponseWriter, r *http.Request, p *Playback, index int) {
	rc, size, err := p.stream.Segment(r.Context(), index)
	switch {
	case errors.Is(err, streaming.ErrSegmentRange):
		http.NotFound(w, r)
		return
	case errors.Is(err, context.Canceled):
		return
	case err != nil:
		m.log.ErrorContext(r.Context(), "serve segment", "playback", p.ID, "segment", index, "err", err)
		http.Error(w, "transcode failed", http.StatusInternalServerError)
		return
	}
	defer func() { _ = rc.Close() }()
	contentType := "video/mp4"
	if p.segExt == "ts" {
		contentType = "video/mp2t"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	if _, err := io.Copy(w, rc); err != nil {
		m.log.DebugContext(r.Context(), "segment interrupted", "playback", p.ID, "segment", index, "err", err)
	}
}

// serveFile serves the source file as is, with range requests, opened
// within its library folder.
func (m *Manager) serveFile(w http.ResponseWriter, r *http.Request, p *Playback) {
	f, info, err := openInRoot(p.root, p.Source().Path)
	if err != nil {
		m.log.ErrorContext(r.Context(), "open media", "playback", p.ID, "err", err)
		http.Error(w, "media unavailable", http.StatusNotFound)
		return
	}
	defer func() { _ = f.Close() }()
	if t, ok := mediaTypes[strings.ToLower(filepath.Ext(info.Name()))]; ok {
		w.Header().Set("Content-Type", t)
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

func openInRoot(root, path string) (*os.File, os.FileInfo, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.OpenInRoot(root, rel)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, nil, fmt.Errorf("%s is not a regular file", path)
	}
	return f, info, nil
}

// attachment names a download after its item.
func attachment(p *Playback, ext string) string {
	name := strings.Map(func(r rune) rune {
		if r < ' ' || strings.ContainsRune(`"\\/:*?<>|`, r) {
			return '_'
		}
		return r
	}, p.Item.Name)
	if name == "" {
		name = "download"
	}
	return `attachment; filename*=UTF-8''` + url.PathEscape(name+ext)
}

// serveProgressive runs ffmpeg for the request, writing its output as the
// response; such a stream cannot be seeked.
func (m *Manager) serveProgressive(w http.ResponseWriter, r *http.Request, p *Playback) {
	ctype := mediaTypes["."+p.progExt]
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Accept-Ranges", "none")
	if p.Download {
		w.Header().Set("Content-Disposition", attachment(p, "."+p.progExt))
	}
	if r.Method == http.MethodHead {
		return
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(r.Context(), m.cfg.FFmpegPath, p.planner.ProgressiveArgs(p.progressive, "pipe:1")...)
	cmd.Stdout = &touchingWriter{w: w, p: p}
	cmd.Stderr = &limitedBuffer{b: &stderr, max: 64 << 10}
	if err := cmd.Run(); err != nil && r.Context().Err() == nil {
		m.log.ErrorContext(r.Context(), "progressive transcode failed", "playback", p.ID, "err", err,
			"ffmpeg", strings.TrimSpace(stderr.String()))
	}
}

// touchingWriter keeps a playback alive while its stream is written.
type touchingWriter struct {
	w io.Writer
	p *Playback
}

func (t *touchingWriter) Write(b []byte) (int, error) {
	t.p.touch(time.Now())
	return t.w.Write(b)
}

// limitedBuffer keeps the first bytes written.
type limitedBuffer struct {
	b   *bytes.Buffer
	max int
}

func (l *limitedBuffer) Write(b []byte) (int, error) {
	if room := l.max - l.b.Len(); room > 0 {
		l.b.Write(b[:min(len(b), room)])
	}
	return len(b), nil
}
