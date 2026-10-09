package playback

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/media/decision"
	"github.com/mavioai/mavio/libs/media/supervisor"
	"github.com/mavioai/mavio/libs/store"
)

// env is a store with one movie, a user signed in and a manager without
// ffmpeg.
type env struct {
	store *store.Store
	m     *Manager
	user  core.User
	sess  core.AuthSession
	movie core.Item
	// source is the movie's media source.
	source core.ID
	file   []byte
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	root := t.TempDir()
	lib := core.Library{Name: "Movies", Kind: core.LibraryMovies, Paths: []string{root}}
	if err := s.Libraries().Create(ctx, &lib); err != nil {
		t.Fatal(err)
	}
	e := &env{store: s, file: []byte("not really a movie, but bytes to serve")}
	path := filepath.Join(root, "Film (2020)", "Film.mp4")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, e.file, 0o644); err != nil {
		t.Fatal(err)
	}
	srt := filepath.Join(filepath.Dir(path), "Film.fr.srt")
	if err := os.WriteFile(srt, []byte("1\r\n00:00:01,000 --> 00:00:02,500\r\nBonjour\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.movie = core.Item{ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindMovie, Name: "Film", Path: path, Runtime: 100 * time.Minute}
	if err := s.Items().Upsert(ctx, e.movie); err != nil {
		t.Fatal(err)
	}
	src := core.MediaSource{
		ID: core.NewID(), ItemID: e.movie.ID, Path: path, Container: "mov,mp4,m4a,3gp,3g2,mj2", Size: int64(len(e.file)),
		Duration: 100 * time.Minute, Bitrate: 4_000_000,
		Streams: []core.MediaStream{
			{
				Index: 0, Kind: core.StreamVideo, Codec: "h264", Profile: "High", Level: 41, Width: 1920, Height: 1080, BitDepth: 8,
				PixelFormat: "yuv420p", FrameRate: core.Rational{Num: 24, Den: 1}, Bitrate: 3_800_000,
			},
			{Index: 1, Kind: core.StreamAudio, Codec: "aac", Language: "eng", Channels: 2, SampleRate: 48000, Default: true},
			{Index: 2, Kind: core.StreamAudio, Codec: "aac", Language: "fre", Channels: 2, SampleRate: 48000},
			{Index: 3, Kind: core.StreamSubtitle, Codec: "subrip", Language: "eng"},
			{Index: 4, Kind: core.StreamSubtitle, Codec: "subrip", Language: "fre", ExternalPath: srt},
		},
	}
	if err := s.MediaSources().Replace(ctx, e.movie.ID, []core.MediaSource{src}); err != nil {
		t.Fatal(err)
	}
	e.source = src.ID
	e.user = core.User{Name: "alice", PasswordHash: "$argon2id$x", Policy: core.UserPolicy{AllowTranscoding: true}}
	if err := s.Users().Create(ctx, &e.user); err != nil {
		t.Fatal(err)
	}
	e.sess = core.AuthSession{UserID: e.user.ID, TokenHash: make([]byte, 32), DeviceID: "browser"}
	if err := s.AuthSessions().Create(ctx, &e.sess); err != nil {
		t.Fatal(err)
	}
	e.m = NewManager(Config{Store: s, Dir: t.TempDir()})
	return e
}

// mp4Client plays H.264 and AAC in MP4 directly, with external WebVTT.
var mp4Client = &decision.ClientCapabilities{
	Name:       "test",
	DirectPlay: []decision.DirectPlayProfile{{Kind: decision.Video, Container: "mp4,m4v", VideoCodec: "h264", AudioCodec: "aac"}},
	Subtitles:  []decision.SubtitleProfile{{Format: "vtt", Method: decision.SubtitleExternal}},
}

func (e *env) start(t *testing.T, r Request) *Playback {
	t.Helper()
	if r.User.ID.IsZero() {
		r.User = e.user
	}
	if r.ItemID.IsZero() {
		r.ItemID = e.movie.ID
	}
	if r.Client == nil {
		r.Client = mp4Client
	}
	r.SessionID = e.sess.ID
	p, err := e.m.Start(t.Context(), r)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return p
}

func (e *env) userData(t *testing.T) core.UserData {
	t.Helper()
	d, err := e.store.UserData().Get(t.Context(), e.user.ID, e.movie.ID)
	if err != nil && !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
	return d
}

func TestDirectPlay(t *testing.T) {
	e := newEnv(t)
	p := e.start(t, Request{})
	// The default subtitle mode plays external subtitles.
	if p.Method != decision.DirectPlay || p.HLS() || p.AudioStream != 1 || p.SubtitleStream != 4 {
		t.Fatalf("got = %s, audio %d, subtitle %d; want direct play of audio 1 with subtitle 4", p.Method, p.AudioStream, p.SubtitleStream)
	}
	// Without ffmpeg, only the external file is offered as a file.
	if len(p.Subtitles) != 2 || p.Subtitles[0].Method != decision.SubtitleDrop ||
		p.Subtitles[1].Method != decision.SubtitleExternal || p.Subtitles[1].Format != "vtt" {
		t.Errorf("subtitles = %+v, want the external file as vtt", p.Subtitles)
	}

	srv := httptest.NewServer(e.m.Handler())
	t.Cleanup(srv.Close)
	if p.URL() != "media/"+p.ID+"/stream.mp4" {
		t.Errorf("URL = %q", p.URL())
	}
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/"+p.URL(), nil)
	req.Header.Set("Range", "bytes=4-10")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent || string(body) != string(e.file[4:11]) || resp.Header.Get("Content-Type") != "video/mp4" {
		t.Errorf("range request = %d %q (%s), want 206 %q", resp.StatusCode, body, resp.Header.Get("Content-Type"), e.file[4:11])
	}
	sub := p.SubtitleURL(&p.Subtitles[1])
	if sub != "media/"+p.ID+"/subtitles/4.vtt" {
		t.Fatalf("subtitle URL = %q", sub)
	}
	resp, err = http.Get(srv.URL + "/" + sub)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if want := "WEBVTT\n\n00:00:01.000 --> 00:00:02.500\nBonjour\n\n"; resp.StatusCode != http.StatusOK || string(body) != want || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/vtt") {
		t.Errorf("subtitle = %d %q (%s), want %q", resp.StatusCode, body, resp.Header.Get("Content-Type"), want)
	}
	for _, path := range []string{"/media/unknown/stream.mp4", "/" + strings.TrimSuffix(sub, "vtt") + "srt", "/media/" + p.ID + "/subtitles/3.vtt", "/media/" + p.ID + "/subtitles/04.vtt", "/media/" + p.ID + "/master.m3u8", "/media/" + p.ID + "/stream.mkv"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, resp.StatusCode)
		}
	}
}

func TestProgressAndStop(t *testing.T) {
	ctx := t.Context()
	e := newEnv(t)
	changes := 0
	e.m.cfg.OnChange = func(user, session core.ID) {
		if user != e.user.ID || session != e.sess.ID {
			t.Errorf("change of user %v session %v", user, session)
		}
		changes++
	}
	p := e.start(t, Request{SourceID: e.source, AudioStream: new(2), SubtitleStream: new(3)})
	if changes != 1 || e.m.NowPlaying(e.sess.ID) != p || e.m.NowPlaying(core.NewID()) != nil {
		t.Errorf("after start: %d changes, now playing %v", changes, e.m.NowPlaying(e.sess.ID))
	}
	if d := e.userData(t); d.AudioStream == nil || *d.AudioStream != 2 || d.SubtitleStream == nil || *d.SubtitleStream != 3 {
		t.Errorf("remembered streams = %v, %v; want 2, 3", d.AudioStream, d.SubtitleStream)
	}
	// Pausing is a change, reporting while paused not.
	for range 2 {
		if err := e.m.Progress(ctx, e.user.ID, p.ID, 30*time.Minute, true); err != nil {
			t.Fatal(err)
		}
	}
	if st := p.State(); changes != 2 || !st.Paused || st.Position != 30*time.Minute {
		t.Errorf("after pausing: %d changes, state %+v", changes, st)
	}
	if d := e.userData(t); d.Position != 30*time.Minute || d.Played {
		t.Errorf("after progress: position %v, played %v", d.Position, d.Played)
	}
	if err := e.m.Progress(ctx, core.NewID(), p.ID, time.Minute, false); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("progress of another user's playback: %v, want ErrNotFound", err)
	}
	// Reports past the end count the play once.
	for _, pos := range []time.Duration{95 * time.Minute, 97 * time.Minute} {
		if err := e.m.Progress(ctx, e.user.ID, p.ID, pos, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.m.Stop(ctx, e.user.ID, p.ID, new(98*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if d := e.userData(t); !d.Played || d.PlayCount != 1 || d.Position != 0 || d.LastPlayedAt == nil {
		t.Errorf("after stop: %+v; want played once without a position", d)
	}
	if changes != 4 || e.m.NowPlaying(e.sess.ID) != nil {
		t.Errorf("after stop: %d changes, now playing %v", changes, e.m.NowPlaying(e.sess.ID))
	}
	if err := e.m.Stop(ctx, e.user.ID, p.ID, nil); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("second Stop: %v, want ErrNotFound", err)
	}

	// The next playback starts with the remembered streams.
	next := e.start(t, Request{})
	if next.AudioStream != 2 {
		t.Errorf("next playback audio = %d, want the remembered 2", next.AudioStream)
	}
}

func TestStartErrors(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	hevcOnly := &decision.ClientCapabilities{
		Name:        "hevc",
		DirectPlay:  []decision.DirectPlayProfile{{Kind: decision.Video, Container: "mp4", VideoCodec: "hevc", AudioCodec: "aac"}},
		Transcoding: []decision.TranscodingProfile{{Kind: decision.Video, Context: decision.Streaming, Protocol: decision.HTTP, Container: "mkv", VideoCodec: "hevc", AudioCodec: "aac"}},
	}
	tests := []struct {
		name string
		r    Request
		want error
	}{
		{"unknown item", Request{ItemID: core.NewID()}, core.ErrNotFound},
		{"other library", Request{User: core.User{ID: e.user.ID, Policy: core.UserPolicy{Libraries: []core.ID{core.NewID()}}}}, core.ErrNotFound},
		{"rating", Request{User: core.User{ID: e.user.ID, Policy: core.UserPolicy{MaxParentalRating: new(1), BlockUnrated: true}}}, core.ErrNotFound},
		{"no ffmpeg", Request{Client: hevcOnly}, ErrNoSource},
		{"stream without source", Request{AudioStream: new(1), SourceID: core.NilID}, core.ErrInvalid},
	}
	for _, tt := range tests {
		r := tt.r
		if r.User.ID.IsZero() {
			r.User = e.user
		}
		if r.ItemID.IsZero() {
			r.ItemID = e.movie.ID
		}
		if r.Client == nil {
			r.Client = mp4Client
		}
		if _, err := e.m.Start(ctx, r); !errors.Is(err, tt.want) {
			t.Errorf("%s: Start = %v, want %v", tt.name, err, tt.want)
		}
	}

	// Progressive transcodes are not served.
	e.m.cfg.FFmpeg = func(_ context.Context, _ []string) (supervisor.Process, error) { return nil, errors.New("unused") }
	if _, err := e.m.Start(ctx, Request{User: e.user, ItemID: e.movie.ID, Client: hevcOnly}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("progressive transcode: Start = %v, want ErrUnsupported", err)
	}

	limited := e.user
	limited.Policy.MaxSessions = 1
	e.start(t, Request{User: limited})
	if _, err := e.m.Start(ctx, Request{User: limited, ItemID: e.movie.ID, Client: mp4Client}); !errors.Is(err, ErrTooManyPlaybacks) {
		t.Errorf("second playback: Start = %v, want ErrTooManyPlaybacks", err)
	}
}

// A remux of video without keyframes encodes the video instead, and
// queues their extraction.
func TestNoKeyframes(t *testing.T) {
	e := newEnv(t)
	e.m.cfg.FFmpeg = func(context.Context, []string) (supervisor.Process, error) { return nil, errors.New("unused") }
	hls := &decision.ClientCapabilities{
		Name: "hls",
		Transcoding: []decision.TranscodingProfile{{
			Kind: decision.Video, Context: decision.Streaming, Protocol: decision.HLS, Container: "mp4", VideoCodec: "h264", AudioCodec: "aac",
		}},
		// Subtitles that would be burned in force encoding anyway.
		Subtitles: []decision.SubtitleProfile{{Format: "vtt", Method: decision.SubtitleExternal}},
	}
	p := e.start(t, Request{Client: hls})
	if !p.HLS() || p.Method != decision.Transcode || p.variant.Codecs[0] == "" {
		t.Errorf("got = %s, HLS %v; want a transcode", p.Method, p.HLS())
	}
	job := library.KeyframesJob(e.movie.ID, time.Now(), library.KeyframesUrgent)
	if added, err := e.store.Jobs().Enqueue(t.Context(), &job); err != nil || added {
		t.Errorf("keyframe job queued again = %v, %v; want it pending already", added, err)
	}
}

func TestCheck(t *testing.T) {
	ctx := t.Context()
	e := newEnv(t)
	idle := e.start(t, Request{Start: 40 * time.Minute})
	active := e.start(t, Request{})
	idle.lastActive = time.Now().Add(-2 * e.m.cfg.IdleTimeout)

	e.m.check(ctx)
	if e.m.Get(idle.ID) != nil || e.m.Get(active.ID) == nil {
		t.Fatalf("after check: idle kept %v, active kept %v; want only the active one", e.m.Get(idle.ID) != nil, e.m.Get(active.ID) != nil)
	}
	if d := e.userData(t); d.Position != 40*time.Minute {
		t.Errorf("expired playback position = %v, want 40m", d.Position)
	}

	// Signing out ends the session's playbacks.
	if err := e.store.AuthSessions().Delete(ctx, e.sess.ID); err != nil {
		t.Fatal(err)
	}
	e.m.check(ctx)
	if e.m.Get(active.ID) != nil {
		t.Error("playback of a signed-out session kept")
	}
}
