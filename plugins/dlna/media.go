package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/plugin/guest"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1/playbackv1connect"
	"github.com/mavioai/mavio/plugins/dlna/internal/didl"
	"github.com/mavioai/mavio/plugins/dlna/internal/profile"
)

// stream is a playback a device reads.
type stream struct {
	user, device string
	item         *libraryv1.Item
	resp         *playbackv1.StartPlaybackResponse
	// kind is "video" or "audio".
	kind     string
	direct   bool
	mime     string
	ext      string
	duration time.Duration
	// subtitles tells whether the source has text subtitles.
	subtitles bool
}

// start returns where the stream begins within the item.
func (s *stream) start() time.Duration { return s.resp.GetStartPosition().AsDuration() }

// startStream starts playing an item for a profile, acting as a user and,
// when set, a device.
func (p *plugin) startStream(ctx context.Context, user, device, itemID string, prof *profile.Profile, start time.Duration) (*stream, error) {
	client := hostClient(user, device)
	got, err := libraryv1connect.NewItemServiceClient(client, guest.HostURL).GetItem(ctx, libraryv1.GetItemRequest_builder{Id: proto.String(itemID)}.Build())
	if err != nil {
		return nil, err
	}
	kind := mediaKind(got.GetItem().GetKind())
	if kind != "video" && kind != "audio" || len(got.GetMediaSources()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%q is not playable", got.GetItem().GetName()))
	}
	req := playbackv1.StartPlaybackRequest_builder{ItemId: proto.String(itemID), Capabilities: prof.ClientCapabilities()}
	if start > 0 {
		req.StartPosition = durationpb.New(start)
	}
	playbacks := playbackv1connect.NewPlaybackServiceClient(client, guest.HostURL)
	resp, err := playbacks.StartPlayback(ctx, req.Build())
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(resp.GetUrl(), ".m3u8") {
		_, _ = playbacks.StopPlayback(ctx, playbackv1.StopPlaybackRequest_builder{PlaybackId: proto.String(resp.GetPlaybackId())}.Build())
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("profile %q streams HLS, which DLNA devices do not play", prof.Name))
	}
	ms := got.GetMediaSources()[0]
	for _, m := range got.GetMediaSources() {
		if m.GetId() == resp.GetMediaSourceId() {
			ms = m
		}
	}
	s := &stream{
		user: user, device: device, item: got.GetItem(), resp: resp, kind: kind,
		direct:    resp.GetMethod() == playbackv1.PlayMethod_PLAY_METHOD_DIRECT_PLAY,
		duration:  ms.GetDuration().AsDuration(),
		subtitles: hasTextSubtitles(ms),
	}
	container := strings.TrimPrefix(path.Ext(resp.GetUrl()), ".")
	if s.direct && ms.GetContainer() != "" {
		container = ms.GetContainer()
	}
	s.mime, s.ext = prof.MimeType(container, kind), extension(container)
	return s, nil
}

// stop ends a stream's playback at a position; a negative one means it
// played to the end.
func (s *stream) stop(ctx context.Context, position time.Duration) error {
	req := playbackv1.StopPlaybackRequest_builder{PlaybackId: proto.String(s.resp.GetPlaybackId())}
	if position >= 0 {
		req.Position = durationpb.New(position)
	}
	_, err := playbackv1connect.NewPlaybackServiceClient(hostClient(s.user, s.device), guest.HostURL).StopPlayback(ctx, req.Build())
	return err
}

// report tells the server where a stream's playback is.
func (s *stream) report(ctx context.Context, position time.Duration, paused bool) error {
	_, err := playbackv1connect.NewPlaybackServiceClient(hostClient(s.user, s.device), guest.HostURL).ReportProgress(ctx,
		playbackv1.ReportProgressRequest_builder{
			PlaybackId: proto.String(s.resp.GetPlaybackId()), Position: durationpb.New(max(position, 0)), Paused: proto.Bool(paused),
		}.Build())
	return err
}

// mediaIdle is how long the media server keeps a playback no request
// reads, within the server's own idle timeout.
const mediaIdle = 4 * time.Minute

// mediaCache keeps the playbacks the media server starts for the devices
// reading its media, so that a device's requests, ranges and all, read
// one playback.
type mediaCache struct {
	p       *plugin
	mu      sync.Mutex
	entries map[mediaKey]*mediaEntry
}

type mediaKey struct {
	user, item, profile string
	start               time.Duration
}

type mediaEntry struct {
	ready chan struct{}
	s     *stream
	err   error
	used  time.Time
}

func newMediaCache(p *plugin) *mediaCache {
	return &mediaCache{p: p, entries: map[mediaKey]*mediaEntry{}}
}

// get returns the stream of a key, starting it unless it runs.
func (c *mediaCache) get(ctx context.Context, key mediaKey, prof *profile.Profile) (*mediaEntry, error) {
	now := time.Now()
	c.mu.Lock()
	for k, e := range c.entries {
		if now.Sub(e.used) > mediaIdle {
			delete(c.entries, k)
		}
	}
	e, ok := c.entries[key]
	if !ok {
		e = &mediaEntry{ready: make(chan struct{})}
		c.entries[key] = e
	}
	e.used = now
	c.mu.Unlock()
	if !ok {
		// Other requests wait for the playback; the first one's end must
		// not end it.
		sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		e.s, e.err = c.p.startStream(sctx, key.user, "", key.item, prof, key.start)
		cancel()
		close(e.ready)
		if e.err != nil {
			c.drop(key, e)
		}
		return e, e.err
	}
	select {
	case <-e.ready:
		return e, e.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// drop forgets an entry, as when its playback has ended.
func (c *mediaCache) drop(key mediaKey, e *mediaEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries[key] == e {
		delete(c.entries, key)
	}
}

// profileFor returns the profile a request names, else the one matching
// its headers.
func profileFor(set *profile.Set, r *http.Request) *profile.Profile {
	if name := r.URL.Query().Get("profile"); name != "" {
		if prof, ok := set.Get(name); ok {
			return prof
		}
	}
	return set.Match(profile.Device{Header: r.Header})
}

// serveMedia serves the media of an item to a device browsing the media
// server, in the format of its profile, starting where TimeSeekRange.dlna.org
// asks.
func (p *plugin) serveMedia(w http.ResponseWriter, r *http.Request) {
	c, set, _, ready := p.state()
	if !ready || !c.mediaServer() {
		http.NotFound(w, r)
		return
	}
	prof := profileFor(set, r)
	start, seek := parseTimeSeek(r.Header.Get("TimeSeekRange.dlna.org"))
	key := mediaKey{user: c.User, item: r.PathValue("item"), profile: prof.Name, start: start}
	for range 2 {
		e, err := p.media.get(r.Context(), key, prof)
		if err != nil {
			writeError(w, err)
			return
		}
		s := e.s
		if seek && !s.direct {
			w.Header().Set("TimeSeekRange.dlna.org", "npt="+npt(s.start())+"-"+npt(s.duration)+"/"+npt(s.duration))
		}
		if r.Header.Get("getcaptionInfo.sec") == "1" && prof.CaptionInfo && s.kind == "video" && s.subtitles {
			w.Header().Set("CaptionInfo.sec", p.requestRoot(r)+"/subtitles/"+url.PathEscape(key.item)+"/subtitles.srt?profile="+url.QueryEscape(prof.Name))
		}
		if !p.proxyStream(w, r, s, prof) {
			return
		}
		// The playback has ended, idle or stopped; start another.
		p.media.drop(key, e)
	}
	http.Error(w, "the playback ended", http.StatusServiceUnavailable)
}

// servePlayTo serves the media of a playback Play To started on a
// renderer.
func (p *plugin) servePlayTo(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	rs := p.renderers
	p.mu.Unlock()
	s, prof, ok := rs.stream(r.PathValue("playback"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	if p.proxyStream(w, r, s, prof) {
		http.NotFound(w, r)
	}
}

// proxyHeaders are the headers of media requests and responses passed
// between devices and the server.
var (
	proxyRequestHeaders  = []string{"Range", "If-Range", "If-Modified-Since", "If-None-Match"}
	proxyResponseHeaders = []string{"Content-Length", "Content-Range", "Accept-Ranges", "Last-Modified", "Etag", "Location", "Cache-Control"}
)

// proxyStream serves a stream's media from the server with DLNA's
// headers; it reports whether the playback is gone, before writing
// anything.
func (p *plugin) proxyStream(w http.ResponseWriter, r *http.Request, s *stream, prof *profile.Profile) (gone bool) {
	resp, err := p.upstream(r, s.resp.GetUrl())
	if err != nil {
		writeError(w, err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return true
	}
	h := w.Header()
	copyHeaders(h, resp.Header, proxyResponseHeaders)
	if resp.StatusCode < 300 {
		h.Set("Content-Type", s.mime)
	}
	h.Set("transferMode.dlna.org", "Streaming")
	h.Set("contentFeatures.dlna.org", contentFeatures(s.direct, prof.TimeSeek && !s.direct))
	w.WriteHeader(resp.StatusCode)
	if r.Method != http.MethodHead {
		if _, err := io.Copy(w, resp.Body); err != nil {
			p.log.DebugContext(r.Context(), "media interrupted", "playback", s.resp.GetPlaybackId(), "err", err)
		}
	}
	return false
}

// upstream requests a path of the server, relative to its base URL, with
// a request's conditional and range headers.
func (p *plugin) upstream(r *http.Request, rel string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, guest.HostURL+"/"+strings.TrimPrefix(rel, "/"), nil)
	if err != nil {
		return nil, err
	}
	copyHeaders(req.Header, r.Header, proxyRequestHeaders)
	return hostClient("", "").Do(req)
}

func copyHeaders(dst, src http.Header, names []string) {
	for _, k := range names {
		if v := src.Values(k); len(v) > 0 {
			dst[http.CanonicalHeaderKey(k)] = v
		}
	}
}

// serveSubtitles serves the text subtitles of an item as a profile reads
// them beside the media.
func (p *plugin) serveSubtitles(w http.ResponseWriter, r *http.Request) {
	c, set, _, ready := p.state()
	if !ready || !c.mediaServer() {
		http.NotFound(w, r)
		return
	}
	prof := profileFor(set, r)
	key := mediaKey{user: c.User, item: r.PathValue("item"), profile: prof.Name}
	for range 2 {
		e, err := p.media.get(r.Context(), key, prof)
		if err != nil {
			writeError(w, err)
			return
		}
		var track *playbackv1.SubtitleTrack
		for _, t := range e.s.resp.GetSubtitles() {
			if t.GetMethod() != playbackv1.SubtitleMethod_SUBTITLE_METHOD_EXTERNAL || t.GetUrl() == "" {
				continue
			}
			if track == nil || t.GetStreamIndex() == e.s.resp.GetSubtitleStreamIndex() {
				track = t
			}
		}
		if track == nil {
			http.NotFound(w, r)
			return
		}
		resp, err := p.upstream(r, track.GetUrl())
		if err != nil {
			writeError(w, err)
			return
		}
		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			p.media.drop(key, e)
			continue
		}
		copyHeaders(w.Header(), resp.Header, []string{"Content-Type", "Content-Length", "Last-Modified", "Etag"})
		if track.GetFormat() == "srt" || track.GetFormat() == "subrip" {
			w.Header().Set("Content-Type", "text/srt; charset=utf-8")
		}
		w.WriteHeader(resp.StatusCode)
		if r.Method != http.MethodHead {
			_, _ = io.Copy(w, resp.Body)
		}
		resp.Body.Close()
		return
	}
	http.Error(w, "the playback ended", http.StatusServiceUnavailable)
}

// serveImage serves an image in JPEG, at most size pixels wide and high.
func (p *plugin) serveImage(w http.ResponseWriter, r *http.Request) {
	size, err := strconv.Atoi(r.URL.Query().Get("size"))
	if err != nil || size <= 0 || size > 4096 {
		size = 4096
	}
	q := url.Values{"format": {"jpg"}, "maxWidth": {strconv.Itoa(size)}, "maxHeight": {strconv.Itoa(size)}}
	resp, err := p.upstream(r, "images/"+url.PathEscape(r.PathValue("image"))+"?"+q.Encode())
	if err != nil {
		writeError(w, err)
		return
	}
	defer resp.Body.Close()
	copyHeaders(w.Header(), resp.Header, []string{"Content-Type", "Content-Length", "Last-Modified", "Etag", "Cache-Control"})
	pn := "JPEG_LRG"
	if size <= 160 {
		pn = "JPEG_TN"
	}
	w.Header().Set("transferMode.dlna.org", "Interactive")
	w.Header().Set("contentFeatures.dlna.org", imageFeatures(pn))
	w.WriteHeader(resp.StatusCode)
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, resp.Body)
	}
}

// writeError answers a request with an error of the host API.
func writeError(w http.ResponseWriter, err error) {
	switch connect.CodeOf(err) {
	case connect.CodeNotFound, connect.CodeInvalidArgument:
		http.Error(w, err.Error(), http.StatusNotFound)
	case connect.CodeFailedPrecondition, connect.CodePermissionDenied:
		http.Error(w, err.Error(), http.StatusForbidden)
	default:
		if errors.Is(err, context.Canceled) {
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
}

// parseTimeSeek reads the start of TimeSeekRange.dlna.org, "npt=S-[E]",
// with S in seconds or H:MM:SS[.F].
func parseTimeSeek(h string) (time.Duration, bool) {
	v, ok := strings.CutPrefix(strings.TrimSpace(h), "npt=")
	if !ok {
		return 0, false
	}
	start, _, ok := strings.Cut(v, "-")
	if !ok {
		return 0, false
	}
	start = strings.TrimSpace(start)
	if strings.Contains(start, ":") {
		d, err := didl.ParseDuration(start)
		return d, err == nil
	}
	f, err := strconv.ParseFloat(start, 64)
	if err != nil || f < 0 {
		return 0, false
	}
	return time.Duration(f * float64(time.Second)), true
}

// npt writes a duration in seconds, as TimeSeekRange.dlna.org does.
func npt(d time.Duration) string { return strconv.FormatFloat(d.Seconds(), 'f', 3, 64) }
