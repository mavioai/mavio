package playback

import (
	"cmp"
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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	// ErrNotAllowed is returned for downloads the user's policy forbids.
	ErrNotAllowed = errors.New("playback: downloads are not allowed")
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
	// Record logs playbacks starting and stopping; nil logs none.
	Record func(ctx context.Context, a core.Activity)
	// Device reports whether a session is a plugin's device, which has no
	// sign-in: its playbacks end when the plugin no longer lists it. Nil
	// means no session is.
	Device func(session core.ID) bool
	// QuietGate makes background library I/O yield: starting a playback
	// and every media request, a seek included, extend its quiet window,
	// as khuaplayer's foreground storage gate does, so that streaming keeps
	// background reads paused while it reads and lets them resume between.
	QuietGate *storage.QuietGate
	// Keeper keeps the volumes of the playbacks in progress awake; nil
	// leaves them be.
	Keeper *storage.Keeper
	// RemoteSources gives an item's media sources read over HTTP, which
	// playbacks consider after its files; nil gives none.
	RemoteSources func(ctx context.Context, it core.Item) []core.MediaSource
	Logger        *slog.Logger
}

// Manager runs the playbacks in progress.
type Manager struct {
	cfg Config
	log *slog.Logger

	// planner and dir are what playbacks starting now use; settings
	// replace them while others run.
	planner atomic.Pointer[planner.Planner]
	dir     atomic.Pointer[string]

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
	m := &Manager{cfg: cfg, log: log, playbacks: map[string]*Playback{}}
	m.planner.Store(cfg.Planner)
	m.dir.Store(&cfg.Dir)
	return m
}

// Accelerations lists the hardware accelerations this server can use.
func (m *Manager) Accelerations() []core.HardwareAcceleration {
	out := []core.HardwareAcceleration{core.HardwareAuto, core.HardwareNone}
	if caps := m.cfg.Planner.Caps; caps != nil && caps.SupportsHwaccel("videotoolbox") {
		out = append(out, core.HardwareVideoToolbox)
	}
	return out
}

// SetTranscoding applies transcoding settings to the playbacks that start
// from now on; running ones keep theirs. An empty transcode folder keeps
// the one the manager started with.
func (m *Manager) SetTranscoding(t core.TranscodingSettings) error {
	if !slices.Contains(m.Accelerations(), t.HardwareAcceleration) {
		return fmt.Errorf("%w: hardware acceleration %s is unavailable", core.ErrInvalid, t.HardwareAcceleration)
	}
	dir := cmp.Or(t.TranscodeDir, m.cfg.Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%w: transcode folder: %w", core.ErrInvalid, err)
	}
	opts := m.cfg.Planner.Options
	opts.Hardware = ""
	if t.HardwareAcceleration == core.HardwareVideoToolbox ||
		t.HardwareAcceleration == core.HardwareAuto && slices.Contains(m.Accelerations(), core.HardwareVideoToolbox) {
		opts.Hardware = "videotoolbox"
	}
	opts.HardwareEncoding, opts.Preset, opts.Threads = t.HardwareEncoding, t.EncoderPreset, t.Threads
	opts.H264CRF, opts.H265CRF = t.H264CRF, t.H265CRF
	opts.TonemapAlgorithm, opts.TonemapRange, opts.TonemapDesat, opts.TonemapPeak = t.TonemapAlgorithm, t.TonemapRange, t.TonemapDesat, t.TonemapPeak
	opts.DeinterlaceMethod, opts.DeinterlaceDoubleRate = t.DeinterlaceMethod, t.DeinterlaceDoubleRate
	opts.DownmixBoost, opts.CropBlackBorders = t.DownmixBoost, t.CropBlackBorders
	pl := *m.cfg.Planner
	pl.Options = opts
	m.planner.Store(&pl)
	m.dir.Store(&dir)
	return nil
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
	// Download asks for a file to keep: the source as it is when the
	// client plays it, else a progressive transcode, never HLS.
	Download bool
}

// Playback is a playback in progress.
type Playback struct {
	ID        string
	UserID    core.ID
	SessionID core.ID
	// UserName names the user in the activity log.
	UserName string
	Item     core.Item
	Decision *decision.Decision
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
	// StartPosition is where the client begins: where it asked to, or a
	// keyframe up to five seconds before.
	StartPosition time.Duration

	// root is the library folder holding the source, for direct play.
	root string
	// Download says the playback is a download.
	Download bool
	// Progressive remuxes and transcodes: the job, the planner that made
	// it, and the output's extension.
	progressive *planner.Job
	planner     *planner.Planner
	progExt     string
	// HLS deliveries.
	stream  *streaming.Stream
	variant streaming.Variant
	layout  streaming.Layout
	segExt  string

	// attachMu serializes extracting attachments.
	attachMu sync.Mutex
	// attachments is the folder of extracted attachments, removed at the
	// end.
	attachments string

	// subs caches the subtitle streams read, by index.
	subsMu sync.Mutex
	subs   map[int]*subtitle.Subtitle

	mu         sync.Mutex
	started    time.Time
	lastActive time.Time
	position   time.Duration
	paused     bool
	// counted says the play was counted, once per playback.
	counted bool
	ended   bool
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

// Remote reports whether the source is read over HTTP: its Path is a URL.
func (p *Playback) Remote() bool { return p.Decision.Source.Remote }

// HLS reports whether the playback is delivered as HLS.
func (p *Playback) HLS() bool { return p.stream != nil }

func (p *Playback) touch(now time.Time) {
	p.mu.Lock()
	p.lastActive = now
	p.mu.Unlock()
}

// sources returns an item's media sources: its files, then those read
// over HTTP; local counts the files.
func (m *Manager) sources(ctx context.Context, item core.Item) (sources []core.MediaSource, local int, err error) {
	sources, err = m.cfg.Store.MediaSources().ListForItem(ctx, item.ID)
	if err != nil {
		return nil, 0, err
	}
	local = len(sources)
	if m.cfg.RemoteSources != nil {
		sources = append(sources, m.cfg.RemoteSources(ctx, item)...)
	}
	return sources, local, nil
}

// Sources returns the media sources a user may play an item from: its
// files, then those read over HTTP, which number len(sources) - local.
func (m *Manager) Sources(ctx context.Context, user *core.User, itemID core.ID) (sources []core.MediaSource, local int, err error) {
	item, err := m.cfg.Store.Items().Get(ctx, itemID)
	if err != nil {
		return nil, 0, err
	}
	if !user.Policy.CanAccess(&item) {
		return nil, 0, fmt.Errorf("item %s: %w", item.ID, core.ErrNotFound)
	}
	return m.sources(ctx, item)
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
	if r.Download && !r.User.Policy.AllowDownload {
		return nil, ErrNotAllowed
	}
	if limit := r.User.Policy.MaxSessions; limit > 0 && m.countFor(r.User.ID) >= limit {
		return nil, ErrTooManyPlaybacks
	}
	lib, err := st.Libraries().Get(ctx, item.LibraryID)
	if err != nil {
		return nil, err
	}
	mediaSources, local, err := m.sources(ctx, item)
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
		s.Remote = i >= local
		s.NoTranscoding = !r.User.Policy.AllowTranscoding || m.cfg.FFmpeg == nil
		s.NoDirectStream = m.cfg.FFmpeg == nil
		sources[i] = s
	}
	maxBitrate := r.MaxBitrate
	if b := r.User.Policy.MaxStreamingBitrate; b > 0 && (maxBitrate == 0 || b < maxBitrate) {
		maxBitrate = b
	}
	use := decision.Streaming
	if r.Download {
		use = decision.Static
	}
	req := &decision.Request{
		Sources: sources, SourceID: r.SourceID, Client: r.Client, Context: use,
		MaxBitrate:       maxBitrate,
		AudioStreamIndex: r.AudioStream, SubtitleStreamIndex: r.SubtitleStream,
		EnableDirectPlay: true, EnableDirectStream: true,
		AllowVideoStreamCopy: true, AllowAudioStreamCopy: true,
	}
	d, err := m.decide(video, req)
	if err != nil {
		return nil, err
	}
	if d != nil && d.Method == decision.DirectStream && d.Protocol != decision.HLS && !r.Download {
		// A progressive remux is the last resort of streaming clients:
		// remux into the client's HLS profile instead when it has one, and
		// into its progressive profile when it does not take the source's
		// container, as DLNA renderers do not.
		req.EnableDirectStream = false
		other, err := m.decide(video, req)
		if err == nil && other != nil && (other.Protocol == decision.HLS || d.Reasons&decision.ContainerNotSupported != 0) {
			d = other
		}
	}
	if d == nil {
		return nil, ErrNoSource
	}

	p := &Playback{
		ID: newID(), UserID: r.User.ID, SessionID: r.SessionID, UserName: r.User.Name, Item: item, Decision: d, Method: d.Method, Download: r.Download,
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
	if p.root = libraryRoot(&lib, ms.Path); p.root == "" && !d.Source.Remote {
		return nil, fmt.Errorf("%w: %s lies outside its library", ErrNotPlayable, ms.Path)
	}
	switch {
	case d.Method == decision.DirectPlay:
	case d.Protocol == decision.HLS && !r.Download:
		if err := m.prepareHLS(ctx, p); err != nil {
			return nil, err
		}
	default:
		if err := m.prepareProgressive(p); err != nil {
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
		m.cfg.QuietGate.NoteActivity()
	}
	p.StartPosition = p.startPosition(r.Start)
	p.position = p.StartPosition
	m.prepareStart(ctx, p)

	p.started = time.Now()
	m.mu.Lock()
	m.playbacks[p.ID] = p
	m.mu.Unlock()
	m.log.InfoContext(ctx, "playback started", "playback", p.ID, "user", r.User.Name, "item", item.Name,
		"method", p.Method, "reasons", d.Reasons.String(), "client", r.Client.Name)
	m.changed(p)
	m.report(ctx, p, "playback.started", p.StartPosition, map[string]string{
		"method": string(p.Method), "client": r.Client.Name,
	})
	return p, nil
}

// report logs a playback starting or stopping at pos.
func (m *Manager) report(ctx context.Context, p *Playback, eventType string, pos time.Duration, attrs map[string]string) {
	if m.cfg.Record == nil {
		return
	}
	verb := "started"
	if eventType == "playback.stopped" {
		verb = "stopped"
	}
	if attrs == nil {
		attrs = map[string]string{}
	}
	attrs["playback"], attrs["session"] = p.ID, p.SessionID.String()
	attrs["position"] = strconv.FormatInt(pos.Milliseconds(), 10)
	m.cfg.Record(ctx, core.Activity{
		Type: eventType, UserID: p.UserID, ItemID: p.Item.ID,
		Title: fmt.Sprintf("%s %s playing %s", p.UserName, verb, p.Item.Name), Attributes: attrs,
	})
}

// stopped logs a playback stopping at pos.
func (m *Manager) stopped(ctx context.Context, p *Playback, pos time.Duration) {
	p.mu.Lock()
	played := p.counted
	p.mu.Unlock()
	m.report(ctx, p, "playback.stopped", pos, map[string]string{"played": strconv.FormatBool(played)})
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

// prepareProgressive sets up a remux or transcode served as one stream,
// which ffmpeg writes as it is requested.
func (m *Manager) prepareProgressive(p *Playback) error {
	if m.cfg.FFmpegPath == "" {
		return fmt.Errorf("%w: no ffmpeg to transcode with", ErrUnsupported)
	}
	pl := m.planner.Load()
	job := pl.Job(p.Decision, 0)
	if job.Container == "" {
		job.Container = cmp.Or(p.Decision.Container, "mp4")
	}
	job.Container, _, _ = strings.Cut(job.Container, ",")
	out := pl.Output(job)
	p.Method = decision.Transcode
	if out.VideoCopied || (job.VideoCodec == "" && out.AudioCopied) {
		p.Method = decision.DirectStream
	}
	p.progressive, p.planner, p.progExt = job, pl, job.Container
	return nil
}

// prepareHLS sets up the stream of a remux or transcode; ffmpeg starts
// with the first segment requested.
func (m *Manager) prepareHLS(ctx context.Context, p *Playback) error {
	d, ms := p.Decision, p.Source()
	pl, dir := m.planner.Load(), *m.dir.Load()
	job := pl.Job(d, 0)
	if job.SegmentLength <= 0 {
		job.SegmentLength = m.cfg.SegmentLength
	}
	if job.SegmentContainer == "" {
		job.SegmentContainer = "mp4"
	}
	out := pl.Output(job)
	if out.VideoCopied && len(ms.Keyframes) == 0 {
		// Copied video is cut at its keyframes, which are not extracted
		// yet: encode this playback's video and extract them for the next.
		job.AllowVideoCopy = false
		job.VideoCodec = "h264"
		if len(d.VideoCodecs) > 0 && d.VideoCodecs[0] != "" {
			job.VideoCodec = d.VideoCodecs[0]
		}
		out = pl.Output(job)
		if !d.Source.Remote {
			kf := library.KeyframesJob(p.Item.ID, time.Now(), library.KeyframesUrgent)
			if _, err := m.cfg.Store.Jobs().Enqueue(context.WithoutCancel(ctx), &kf); err != nil {
				m.log.WarnContext(ctx, "queue keyframe extraction", "item", p.Item.ID, "err", err)
			}
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
	s, err := streaming.NewStream(streaming.Config{
		Dir: filepath.Join(dir, p.ID), Layout: p.layout, Container: job.SegmentContainer,
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
	err = m.record(ctx, p, end)
	m.stopped(ctx, p, end)
	return err
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
	p.mu.Unlock()
	if !ended && p.stream != nil {
		_ = p.stream.Close()
	}
	if !ended {
		p.attachMu.Lock()
		if p.attachments != "" {
			_ = os.RemoveAll(p.attachments)
		}
		p.attachMu.Unlock()
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
	var beats <-chan time.Time
	if m.cfg.Keeper != nil {
		interval := m.cfg.Keeper.Interval
		if interval <= 0 {
			interval = storage.DefaultHeartbeatInterval
		}
		bt := time.NewTicker(interval)
		defer bt.Stop()
		beats = bt.C
	}
	for {
		select {
		case <-beats:
			m.keepAwake()
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
		signedIn := slices.ContainsFunc(list, func(s core.AuthSession) bool { return s.ID == p.SessionID }) ||
			m.cfg.Device != nil && m.cfg.Device(p.SessionID)
		p.mu.Lock()
		idle, pos := now.Sub(p.lastActive) > m.cfg.IdleTimeout, p.position
		p.mu.Unlock()
		switch {
		case !signedIn:
			m.log.InfoContext(ctx, "playback ended with its sign-in", "playback", p.ID)
			m.remove(p)
			m.stopped(ctx, p, pos)
		case idle:
			m.log.InfoContext(ctx, "playback expired", "playback", p.ID, "position", pos)
			m.remove(p)
			if pos > 0 {
				if err := m.record(ctx, p, pos); err != nil {
					m.log.WarnContext(ctx, "record expired playback", "playback", p.ID, "err", err)
				}
			}
			m.stopped(ctx, p, pos)
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
