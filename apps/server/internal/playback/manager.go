package playback

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/library/storage"
	"github.com/mavioai/mavio/libs/media/decision"
	"github.com/mavioai/mavio/libs/media/planner"
	"github.com/mavioai/mavio/libs/media/supervisor"
	"github.com/mavioai/mavio/libs/streaming"
	"github.com/mavioai/mavio/libs/subtitle"
)

// Errors of Start.
var (
	// ErrNotPlayable is returned for items without media or of a kind
	// that does not play.
	ErrNotPlayable = errors.New("playback: item cannot be played")
	// ErrNoSource is returned when no media source plays on the client.
	ErrNoSource = errors.New("playback: no media source plays on this client")
	// ErrUnsupported is returned for decisions the server cannot carry
	// out yet, such as progressive transcodes.
	ErrUnsupported = errors.New("playback: unsupported delivery")
	// ErrTooManyPlaybacks is returned when the user reached their
	// policy's limit of concurrent playbacks.
	ErrTooManyPlaybacks = errors.New("playback: too many concurrent playbacks")
)

// Config configures a Manager.
type Config struct {
	Store core.Store
	// Builder decides how media plays; nil decides for a server without
	// ffmpeg.
	Builder *decision.Builder
	// Planner plans transcodes; nil uses the default options.
	Planner *planner.Planner
	// FFmpeg starts transcodes; nil leaves direct play only.
	FFmpeg supervisor.Starter
	// FFmpegPath runs ffmpeg for subtitle extraction; empty serves only
	// external subtitle files.
	FFmpegPath string
	// PauseKey says the ffmpeg build pauses on "p", see
	// [supervisor.Config].
	PauseKey bool
	// Dir holds transcodes, one directory per playback.
	Dir string
	// IdleTimeout ends playbacks without progress reports or media
	// requests for this long; default five minutes.
	IdleTimeout time.Duration
	// SegmentLength is the HLS segment length when the client asks for
	// none; default six seconds.
	SegmentLength time.Duration
	// CheckInterval is how often idle playbacks and ended sign-ins are
	// looked for; default 30 seconds.
	CheckInterval time.Duration
	// OnChange is told when a session starts or ends a playback, or
	// pauses or resumes it; nil tells no one.
	OnChange func(userID, sessionID core.ID)
	// QuietGate pauses background library scans while playback sessions are active.
	QuietGate *storage.QuietGate
	Logger    *slog.Logger
}

// Manager runs the playbacks in progress.
type Manager struct {
	cfg Config
	log *slog.Logger

	mu        sync.Mutex
	playbacks map[string]*Playback
}

// NewManager returns a manager; Run must run for idle playbacks to end.
func NewManager(cfg Config) *Manager {
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 5 * time.Minute
	}
	if cfg.SegmentLength <= 0 {
		cfg.SegmentLength = 6 * time.Second
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 30 * time.Second
	}
	if cfg.Builder == nil {
		cfg.Builder = &decision.Builder{Transcoder: noTranscoder{}}
	}
	if cfg.Planner == nil {
		cfg.Planner = &planner.Planner{Options: planner.DefaultOptions()}
	}
	log := cfg.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Manager{cfg: cfg, log: log, playbacks: map[string]*Playback{}}
}

// Request asks to play an item.
type Request struct {
	User      core.User
	SessionID core.ID
	ItemID    core.ID
	// SourceID picks a media source; zero lets the decision pick.
	SourceID core.ID
	Client   *decision.ClientCapabilities
	// AudioStream and SubtitleStream select streams; nil plays the
	// defaults, a subtitle index of -1 none.
	AudioStream, SubtitleStream *int
	// MaxBitrate caps the bitrate below the client's and the user's
	// limits; 0 means none.
	MaxBitrate int64
	// Start is where the client begins playing; HLS clients seek there
	// themselves. It is the position kept should the playback expire
	// before reporting progress.
	Start time.Duration
}

// Playback is a playback in progress.
type Playback struct {
	ID        string
	UserID    core.ID
	SessionID core.ID
	Item      core.Item
	Decision  *decision.Decision
	// Method is how the source plays: direct play, a direct stream when
	// the video (or the audio of audio items) is copied, else a
	// transcode.
	Method decision.Method
	// AudioStream is the index of the audio stream played, -1 for none;
	// SubtitleStream that of the subtitle stream, -1 for none.
	AudioStream, SubtitleStream int
	// Subtitles are the source's subtitle streams and how each reaches
	// the client.
	Subtitles []Subtitle

	// root is the library folder holding the source, for direct play.
	root string
	// HLS deliveries.
	stream  *streaming.Stream
	variant streaming.Variant
	layout  streaming.Layout
	segExt  string

	// subs caches the subtitle streams read, by index.
	subsMu sync.Mutex
	subs   map[int]*subtitle.Subtitle

	mu         sync.Mutex
	started    time.Time
	lastActive time.Time
	position   time.Duration
	paused     bool
	// counted says the play was counted, once per playback.
	counted  bool
	ended    bool
	quietRel func()
}

// Subtitle is a subtitle stream of a playback.
type Subtitle struct {
	Stream *core.MediaStream
	Method decision.SubtitleMethod
	// Format is the delivered format, e.g. "vtt".
	Format string
}

// Source is the media source played.
func (p *Playback) Source() *core.MediaSource { return p.Decision.Source.MediaSource }

// HLS reports whether the playback is delivered as HLS.
func (p *Playback) HLS() bool { return p.stream != nil }

func (p *Playback) touch(now time.Time) {
	p.mu.Lock()
	p.lastActive = now
	p.mu.Unlock()
}

// Start decides how the client plays the item and registers the playback.
func (m *Manager) Start(ctx context.Context, r Request) (*Playback, error) {
	st := m.cfg.Store
	item, err := st.Items().Get(ctx, r.ItemID)
	if err != nil {
		return nil, err
	}
	if !r.User.Policy.CanAccess(&item) {
		return nil, fmt.Errorf("item %s: %w", item.ID, core.ErrNotFound)
	}
	var video bool
	switch item.Kind {
	case core.KindMovie, core.KindEpisode, core.KindVideo, core.KindMusicVideo:
		video = true
	case core.KindTrack, core.KindAudioBook:
	default:
		return nil, fmt.Errorf("%w: %s is a %s", ErrNotPlayable, item.ID, item.Kind)
	}
	if limit := r.User.Policy.MaxSessions; limit > 0 && m.countFor(r.User.ID) >= limit {
		return nil, ErrTooManyPlaybacks
	}
	lib, err := st.Libraries().Get(ctx, item.LibraryID)
	if err != nil {
		return nil, err
	}
	mediaSources, err := st.MediaSources().ListForItem(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	if len(mediaSources) == 0 {
		return nil, fmt.Errorf("%w: %s has no media", ErrNotPlayable, item.ID)
	}
	data, err := st.UserData().Get(ctx, r.User.ID, item.ID)
	if err != nil && !errors.Is(err, core.ErrNotFound) {
		return nil, err
	}

	sources := make([]*decision.Source, len(mediaSources))
	for i := range mediaSources {
		s := sourceFor(&mediaSources[i], &r.User.Preferences, &data)
		s.NoTranscoding = !r.User.Policy.AllowTranscoding || m.cfg.FFmpeg == nil
		s.NoDirectStream = m.cfg.FFmpeg == nil
		sources[i] = s
	}
	maxBitrate := r.MaxBitrate
	if b := r.User.Policy.MaxStreamingBitrate; b > 0 && (maxBitrate == 0 || b < maxBitrate) {
		maxBitrate = b
	}
	req := &decision.Request{
		Sources: sources, SourceID: r.SourceID, Client: r.Client, Context: decision.Streaming,
		MaxBitrate:       maxBitrate,
		AudioStreamIndex: r.AudioStream, SubtitleStreamIndex: r.SubtitleStream,
		EnableDirectPlay: true, EnableDirectStream: true,
		AllowVideoStreamCopy: true, AllowAudioStreamCopy: true,
	}
	d, err := m.decide(video, req)
	if err != nil {
		return nil, err
	}
	if d != nil && d.Method == decision.DirectStream && d.Protocol != decision.HLS {
		// A direct stream is a progressive remux, which is not served;
		// remux into the client's HLS profile instead.
		req.EnableDirectStream = false
		if hls, err := m.decide(video, req); err == nil && hls != nil && hls.Protocol == decision.HLS {
			d = hls
		}
	}
	if d == nil {
		return nil, ErrNoSource
	}

	p := &Playback{
		ID: newID(), UserID: r.User.ID, SessionID: r.SessionID, Item: item, Decision: d, Method: d.Method,
		AudioStream: -1, SubtitleStream: -1, lastActive: time.Now(), position: r.Start,
	}
	ms := d.Source.MediaSource
	if a := d.TargetAudioStream(); a != nil {
		p.AudioStream = a.Index
	}
	if i := d.SubtitleStreamIndex; i != nil {
		p.SubtitleStream = *i
	}
	p.Subtitles = m.subtitles(d, r.Client)
	if p.root = libraryRoot(&lib, ms.Path); p.root == "" {
		return nil, fmt.Errorf("%w: %s lies outside its library", ErrNotPlayable, ms.Path)
	}
	if d.Method != decision.DirectPlay {
		if d.Protocol != decision.HLS {
			return nil, fmt.Errorf("%w: progressive %s to %s; declare an HLS transcoding profile", ErrUnsupported, d.Method, d.Container)
		}
		if err := m.prepareHLS(ctx, p); err != nil {
			return nil, err
		}
	}
	if r.AudioStream != nil || r.SubtitleStream != nil {
		// Remember the user's choice for the item's next playback.
		data.UserID, data.ItemID = r.User.ID, item.ID
		if r.AudioStream != nil {
			data.AudioStream = new(p.AudioStream)
		}
		if r.SubtitleStream != nil {
			data.SubtitleStream = new(p.SubtitleStream)
		}
		if err := st.UserData().Put(ctx, &data); err != nil {
			p.close()
			return nil, err
		}
	}

	if m.cfg.QuietGate != nil {
		p.quietRel = m.cfg.QuietGate.AcquirePlayback(p.ID)
	}
	if d.Source != nil && d.Source.Path != "" {
		_ = storage.PrefetchHeadTail(d.Source.Path, 1024*1024, 256*1024)
	}

	p.started = time.Now()
	m.mu.Lock()
	m.playbacks[p.ID] = p
	m.mu.Unlock()
	m.log.InfoContext(ctx, "playback started", "playback", p.ID, "user", r.User.Name, "item", item.Name,
		"method", p.Method, "reasons", d.Reasons.String(), "client", r.Client.Name)
	m.changed(p)
	return p, nil
}

// changed tells OnChange about a playback's session.
func (m *Manager) changed(p *Playback) {
	if m.cfg.OnChange != nil {
		m.cfg.OnChange(p.UserID, p.SessionID)
	}
}

// State is how far a playback is, as last reported.
type State struct {
	Position time.Duration
	Paused   bool
}

// State returns the playback's position and pause as last reported.
func (p *Playback) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return State{Position: p.position, Paused: p.paused}
}

// NowPlaying returns the playback a session started last, nil when it
// plays nothing.
func (m *Manager) NowPlaying(session core.ID) *Playback {
	m.mu.Lock()
	defer m.mu.Unlock()
	var last *Playback
	for _, p := range m.playbacks {
		if p.SessionID == session && (last == nil || p.started.After(last.started)) {
			last = p
		}
	}
	return last
}

// decide decides how to play, returning nil when no source plays.
func (m *Manager) decide(video bool, req *decision.Request) (*decision.Decision, error) {
	var (
		d   *decision.Decision
		err error
	)
	if video {
		d, err = m.cfg.Builder.Video(req)
	} else {
		d, err = m.cfg.Builder.Audio(req)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", core.ErrInvalid, err)
	}
	// Without a transcoding profile that fits, the decision has no output
	// container.
	if d == nil || d.Source == nil || (d.Method != decision.DirectPlay && d.Container == "") {
		return nil, nil
	}
	return d, nil
}

// prepareHLS sets up the stream of a remux or transcode; ffmpeg starts
// with the first segment requested.
func (m *Manager) prepareHLS(ctx context.Context, p *Playback) error {
	d, ms := p.Decision, p.Source()
	job := m.cfg.Planner.Job(d, 0)
	if job.SegmentLength <= 0 {
		job.SegmentLength = m.cfg.SegmentLength
	}
	if job.SegmentContainer == "" {
		job.SegmentContainer = "mp4"
	}
	out := m.cfg.Planner.Output(job)
	if out.VideoCopied && len(ms.Keyframes) == 0 {
		// Copied video is cut at its keyframes, which are not extracted
		// yet: encode this playback's video and extract them for the next.
		job.AllowVideoCopy = false
		job.VideoCodec = "h264"
		if len(d.VideoCodecs) > 0 && d.VideoCodecs[0] != "" {
			job.VideoCodec = d.VideoCodecs[0]
		}
		out = m.cfg.Planner.Output(job)
		kf := library.KeyframesJob(p.Item.ID, time.Now(), library.KeyframesUrgent)
		if _, err := m.cfg.Store.Jobs().Enqueue(context.WithoutCancel(ctx), &kf); err != nil {
			m.log.WarnContext(ctx, "queue keyframe extraction", "item", p.Item.ID, "err", err)
		}
	}
	p.Method = decision.Transcode
	if out.VideoCopied || (job.VideoCodec == "" && out.AudioCopied) {
		p.Method = decision.DirectStream
	}
	if out.VideoCopied {
		p.layout = streaming.CopiedLayout(ms.Keyframes, ms.Duration, job.SegmentLength)
	} else {
		var err error
		if p.layout, err = streaming.EncodedLayout(job.SegmentLength, ms.Duration); err != nil {
			return fmt.Errorf("%w: %w", ErrNotPlayable, err)
		}
	}
	var pixelFormat string
	if job.Video != nil {
		pixelFormat = job.Video.PixelFormat
	}
	p.variant = streaming.VariantOf(out, int(ms.Bitrate), pixelFormat)
	p.segExt = job.SegmentContainer
	pl := m.cfg.Planner
	s, err := streaming.NewStream(streaming.Config{
		Dir: filepath.Join(m.cfg.Dir, p.ID), Layout: p.layout, Container: job.SegmentContainer,
		Args: func(start time.Duration, out planner.HLSOutput) []string {
			j := *job
			j.Start = start
			return pl.HLSArgs(&j, out)
		},
		Start:       m.cfg.FFmpeg,
		Supervision: supervisor.Config{IdleTimeout: m.cfg.IdleTimeout, PauseKey: m.cfg.PauseKey},
		Logger:      m.log.With("playback", p.ID),
	})
	if err != nil {
		return err
	}
	p.stream = s
	return nil
}

// subtitles lists the source's subtitle streams and how each would reach
// the client.
func (m *Manager) subtitles(d *decision.Decision, client *decision.ClientCapabilities) []Subtitle {
	var out []Subtitle
	src := d.Source
	for i := range src.Streams {
		st := &src.Streams[i]
		if st.Kind != core.StreamSubtitle {
			continue
		}
		prof := decision.SubtitleProfileFor(src, st, client.Subtitles, d.Method, m.cfg.Builder.Transcoder, d.Container, d.Protocol)
		sub := Subtitle{Stream: st, Method: prof.Method, Format: prof.Format}
		if d.SubtitleStreamIndex != nil && *d.SubtitleStreamIndex == st.Index && d.SubtitleMethod != "" {
			sub.Method, sub.Format = d.SubtitleMethod, d.SubtitleFormat
		}
		if sub.Method == decision.SubtitleExternal && (!servable(&sub) || (st.ExternalPath == "" && m.cfg.FFmpegPath == "")) {
			// Files the server cannot write, or extract without ffmpeg.
			sub.Method = decision.SubtitleDrop
		}
		out = append(out, sub)
	}
	return out
}

// Get returns the playback with the given ID, nil when there is none.
func (m *Manager) Get(id string) *Playback {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.playbacks[id]
}

func (m *Manager) countFor(userID core.ID) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, p := range m.playbacks {
		if p.UserID == userID {
			n++
		}
	}
	return n
}

// own returns the user's playback with the given ID.
func (m *Manager) own(userID core.ID, id string) (*Playback, error) {
	p := m.Get(id)
	if p == nil || p.UserID != userID {
		return nil, fmt.Errorf("playback %q: %w", id, core.ErrNotFound)
	}
	return p, nil
}

// Progress records the position of one of the user's playbacks and
// whether it is paused.
func (m *Manager) Progress(ctx context.Context, userID core.ID, id string, pos time.Duration, paused bool) error {
	p, err := m.own(userID, id)
	if err != nil {
		return err
	}
	p.touch(time.Now())
	p.mu.Lock()
	toggled := p.paused != paused
	p.paused = paused
	p.mu.Unlock()
	if toggled {
		m.changed(p)
	}
	if p.stream != nil {
		if err := p.stream.ReportPosition(ctx, pos); err != nil {
			return err
		}
	}
	return m.record(ctx, p, pos)
}

// Stop ends one of the user's playbacks at pos; nil means it played to
// the end.
func (m *Manager) Stop(ctx context.Context, userID core.ID, id string, pos *time.Duration) error {
	p, err := m.own(userID, id)
	if err != nil {
		return err
	}
	m.remove(p)
	end := p.Item.Runtime
	if pos != nil {
		end = *pos
	}
	m.log.InfoContext(ctx, "playback stopped", "playback", p.ID, "position", end)
	return m.record(ctx, p, end)
}

// record applies a position to the user's state of the item, counting a
// completed play once per playback.
func (m *Manager) record(ctx context.Context, p *Playback, pos time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.position = pos
	st := m.cfg.Store
	return st.InTx(ctx, func(tx core.Store) error {
		data, err := tx.UserData().Get(ctx, p.UserID, p.Item.ID)
		if errors.Is(err, core.ErrNotFound) {
			data = core.UserData{UserID: p.UserID, ItemID: p.Item.ID}
		} else if err != nil {
			return err
		}
		if data.RecordPosition(&p.Item, pos, time.Now()) && !p.counted {
			p.counted = true
			data.PlayCount++
		}
		return tx.UserData().Put(ctx, &data)
	})
}

// remove unregisters a playback and releases its transcode.
func (m *Manager) remove(p *Playback) {
	m.mu.Lock()
	_, ok := m.playbacks[p.ID]
	delete(m.playbacks, p.ID)
	m.mu.Unlock()
	p.close()
	if ok {
		m.changed(p)
	}
}

func (p *Playback) close() {
	p.mu.Lock()
	ended := p.ended
	p.ended = true
	quietRel := p.quietRel
	p.quietRel = nil
	p.mu.Unlock()
	if quietRel != nil {
		quietRel()
	}
	if !ended && p.stream != nil {
		_ = p.stream.Close()
	}
}

// Run ends idle playbacks and those whose sign-in ended until ctx is
// canceled, then ends all playbacks.
func (m *Manager) Run(ctx context.Context) error {
	if err := os.MkdirAll(m.cfg.Dir, 0o755); err != nil {
		return fmt.Errorf("transcode directory: %w", err)
	}
	t := time.NewTicker(m.cfg.CheckInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			m.mu.Lock()
			all := slices.Collect(maps.Values(m.playbacks))
			clear(m.playbacks)
			m.mu.Unlock()
			for _, p := range all {
				p.close()
			}
			return nil
		case <-t.C:
			m.check(ctx)
		}
	}
}

// check ends playbacks idle for longer than IdleTimeout, saving their last
// position, and those whose sign-in session is gone.
func (m *Manager) check(ctx context.Context) {
	m.mu.Lock()
	all := slices.Collect(maps.Values(m.playbacks))
	m.mu.Unlock()
	sessions := map[core.ID][]core.AuthSession{}
	now := time.Now()
	for _, p := range all {
		list, ok := sessions[p.UserID]
		if !ok {
			var err error
			if list, err = m.cfg.Store.AuthSessions().ListForUser(ctx, p.UserID); err != nil {
				m.log.WarnContext(ctx, "list sessions", "user", p.UserID, "err", err)
				continue
			}
			sessions[p.UserID] = list
		}
		signedIn := slices.ContainsFunc(list, func(s core.AuthSession) bool { return s.ID == p.SessionID })
		p.mu.Lock()
		idle, pos := now.Sub(p.lastActive) > m.cfg.IdleTimeout, p.position
		p.mu.Unlock()
		switch {
		case !signedIn:
			m.log.InfoContext(ctx, "playback ended with its sign-in", "playback", p.ID)
			m.remove(p)
		case idle:
			m.log.InfoContext(ctx, "playback expired", "playback", p.ID, "position", pos)
			m.remove(p)
			if pos > 0 {
				if err := m.record(ctx, p, pos); err != nil {
					m.log.WarnContext(ctx, "record expired playback", "playback", p.ID, "err", err)
				}
			}
		}
	}
}

// libraryRoot returns the library folder holding path, or "".
func libraryRoot(lib *core.Library, path string) string {
	for _, root := range lib.Paths {
		if rel, err := filepath.Rel(root, path); err == nil && filepath.IsLocal(rel) {
			return root
		}
	}
	return ""
}

// noTranscoder stands in for ffmpeg when there is none.
type noTranscoder struct{}

func (noTranscoder) CanEncodeAudio(string) bool      { return false }
func (noTranscoder) CanExtractSubtitles(string) bool { return false }

// newID returns an unguessable playback ID.
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // never fails
	return base64.RawURLEncoding.EncodeToString(b)
}
