package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/guest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	"github.com/mavioai/mavio/plugins/dlna/internal/didl"
	"github.com/mavioai/mavio/plugins/dlna/internal/profile"
	"github.com/mavioai/mavio/plugins/dlna/internal/ssdp"
	"github.com/mavioai/mavio/plugins/dlna/internal/upnp"
)

// The renderer's device and service types.
const (
	rendererType     = "urn:schemas-upnp-org:device:MediaRenderer:1"
	avTransport      = "urn:schemas-upnp-org:service:AVTransport:1"
	renderingControl = "urn:schemas-upnp-org:service:RenderingControl:1"
)

// Play To's timing.
const (
	// searchInterval is how often renderers are searched for, and static
	// ones read.
	searchInterval = 5 * time.Minute
	// searchWait is how long a search waits for responses.
	searchWait = 3 * time.Second
	// defaultMaxAge is how long a renderer is kept without a max-age.
	defaultMaxAge = 30 * time.Minute
	// pollInterval is how often a playing renderer is asked where it is.
	pollInterval = time.Second
	// reportInterval is how often the progress of a playback is reported.
	reportInterval = 5 * time.Second
	// startGrace is how long a renderer may stay stopped after being told
	// to play.
	startGrace = 15 * time.Second
	// endSlack is how near its end a stopped item counts as played.
	endSlack = 10 * time.Second
	// maxFailures is how many polls in a row may fail before the playback
	// is given up.
	maxFailures = 3
)

// renderers is Play To: the DLNA renderers found, listed as the server's
// remote devices, and what they play.
type renderers struct {
	p      *plugin
	cfg    config
	set    *profile.Set
	client *http.Client
	// locations takes the description URLs of renderers to read, with how
	// long they are valid.
	locations chan location
	// dirty is set when the device list has changed since the server
	// last took it.
	dirty atomic.Bool

	mu      sync.Mutex
	devices map[string]*renderer
	reading map[string]bool
}

type location struct {
	url    string
	maxAge time.Duration
	static bool
}

// renderer is a DLNA renderer and what it plays.
type renderer struct {
	r                  *renderers
	udn, name, product string
	location           string
	profile            *profile.Profile
	transport          string
	rendering          string
	// local is the address of this server the renderer reaches.
	local   string
	expires time.Time
	static  bool
	polling atomic.Bool

	// mu serializes the commands and polls of the renderer.
	mu       sync.Mutex
	queue    []string
	index    int
	cur      *stream
	offset   time.Duration
	position time.Duration
	paused   bool
	started  time.Time
	playing  bool
	failures int
	reported time.Time
}

func newRenderers(p *plugin, c config, set *profile.Set) *renderers {
	return &renderers{
		p: p, cfg: c, set: set,
		client:    &http.Client{Timeout: 10 * time.Second},
		locations: make(chan location, 64),
		devices:   map[string]*renderer{},
		reading:   map[string]bool{},
	}
}

// run finds renderers and follows what they play until ctx ends, then
// stops their playbacks and lists them no longer.
func (rs *renderers) run(ctx context.Context, ifaces []net.Interface) {
	var wg sync.WaitGroup
	wg.Go(func() { rs.read(ctx) })
	wg.Go(func() { rs.search(ctx, ifaces) })
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			rs.close()
			return
		case <-tick.C:
		}
		rs.expire()
		if rs.dirty.Load() {
			rs.publish(ctx)
		}
		for _, d := range rs.list() {
			if d.polling.CompareAndSwap(false, true) {
				wg.Go(func() {
					defer d.polling.Store(false)
					d.poll(ctx)
				})
			}
		}
	}
}

// search searches for renderers and reads the static ones now and then.
func (rs *renderers) search(ctx context.Context, ifaces []net.Interface) {
	for {
		for _, u := range rs.cfg.Renderers {
			rs.consider(location{url: u, maxAge: 3 * searchInterval, static: true})
		}
		if len(ifaces) > 0 {
			err := ssdp.Search(ctx, ifaces, rendererType, rs.cfg.SSDPPort, searchWait, func(m ssdp.Message, _ *net.UDPAddr) {
				if loc := m.Header.Get("Location"); loc != "" {
					rs.consider(location{url: loc, maxAge: maxAge(m.Header.Get("Cache-Control"))})
				}
			})
			if err != nil && ctx.Err() == nil {
				rs.p.log.DebugContext(ctx, "search for renderers", "err", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(searchInterval):
		}
	}
}

// notified follows the announcements of renderers.
func (rs *renderers) notified(m ssdp.Message, _ *net.UDPAddr) {
	if !strings.HasPrefix(m.Header.Get("Nt"), strings.TrimSuffix(rendererType, "1")) {
		return
	}
	switch m.Header.Get("Nts") {
	case "ssdp:alive":
		if loc := m.Header.Get("Location"); loc != "" {
			rs.consider(location{url: loc, maxAge: maxAge(m.Header.Get("Cache-Control"))})
		}
	case "ssdp:byebye":
		udn, _, _ := strings.Cut(m.Header.Get("Usn"), "::")
		rs.mu.Lock()
		d, ok := rs.devices[udn]
		if ok && !d.static {
			delete(rs.devices, udn)
			rs.dirty.Store(true)
		}
		rs.mu.Unlock()
		if ok && !d.static {
			go d.gone()
		}
	}
}

// maxAge reads the max-age of a Cache-Control header.
func maxAge(h string) time.Duration {
	for part := range strings.SplitSeq(h, ",") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(strings.ToLower(part)), "max-age"); ok {
			v = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v), "="))
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				return time.Duration(n) * time.Second
			}
		}
	}
	return defaultMaxAge
}

// consider keeps a known renderer at a location, or has the location read.
func (rs *renderers) consider(l location) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for _, d := range rs.devices {
		if d.location == l.url {
			d.expires = time.Now().Add(l.maxAge)
			return
		}
	}
	if rs.reading[l.url] {
		return
	}
	select {
	case rs.locations <- l:
		rs.reading[l.url] = true
	default:
	}
}

// read reads the descriptions of the renderers considered.
func (rs *renderers) read(ctx context.Context) {
	for {
		var l location
		select {
		case <-ctx.Done():
			return
		case l = <-rs.locations:
		}
		d, err := rs.describe(ctx, l)
		rs.mu.Lock()
		delete(rs.reading, l.url)
		if err == nil {
			old, ok := rs.devices[d.udn]
			if ok && old.location == d.location && old.name == d.name && old.profile == d.profile {
				old.expires = d.expires
			} else {
				if ok {
					// The same device moved or changed; keep what it plays.
					old.mu.Lock()
					d.queue, d.index, d.cur, d.offset, d.position = old.queue, old.index, old.cur, old.offset, old.position
					d.paused, d.started, d.playing = old.paused, old.started, old.playing
					old.cur = nil
					old.mu.Unlock()
				}
				rs.devices[d.udn] = d
				rs.dirty.Store(true)
				rs.p.log.InfoContext(ctx, "renderer found", "name", d.name, "product", d.product, "profile", d.profile.Name)
			}
		}
		rs.mu.Unlock()
		if err != nil && ctx.Err() == nil {
			rs.p.log.DebugContext(ctx, "read renderer", "location", l.url, "err", err)
		}
	}
}

// describe reads a renderer's description.
func (rs *renderers) describe(ctx context.Context, l location) (*renderer, error) {
	desc, err := upnp.FetchDescription(ctx, rs.client, l.url)
	if err != nil {
		return nil, err
	}
	dev, ok := desc.Device.Find(rendererType)
	if !ok {
		return nil, errors.New("not a media renderer")
	}
	transport, ok := dev.Service(avTransport)
	if !ok {
		return nil, errors.New("no AVTransport service")
	}
	if dev.UDN == "" || len(dev.UDN) > 256 {
		return nil, errors.New("no usable UDN")
	}
	local, err := localAddress(l.url)
	if err != nil {
		return nil, err
	}
	d := &renderer{
		r: rs, udn: dev.UDN, name: cmp.Or(strings.TrimSpace(dev.FriendlyName), dev.ModelName, "DLNA renderer"),
		product:  strings.TrimSpace(dev.Manufacturer + " " + dev.ModelName),
		location: l.url, transport: transport.ControlURL, local: local,
		expires: time.Now().Add(l.maxAge), static: l.static,
		profile: rs.set.Match(profile.Device{
			FriendlyName: dev.FriendlyName, Manufacturer: dev.Manufacturer, ModelName: dev.ModelName, ModelNumber: dev.ModelNumber,
		}),
	}
	if rc, ok := dev.Service(renderingControl); ok {
		d.rendering = rc.ControlURL
	}
	d.name, d.product = truncate(d.name, 256), truncate(d.product, 256)
	return d, nil
}

// truncate cuts a string to at most n bytes, between runes.
func truncate(s string, n int) string {
	for len(s) > n {
		_, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
	}
	return s
}

// localAddress returns this host's address toward the host of a URL.
func localAddress(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	port := u.Port()
	if port == "" {
		port = "80"
	}
	c, err := net.Dial("udp", net.JoinHostPort(u.Hostname(), port))
	if err != nil {
		return "", fmt.Errorf("route to the renderer: %w", err)
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP.String(), nil
}

// expire forgets the renderers whose announcements have expired.
func (rs *renderers) expire() {
	now := time.Now()
	var gone []*renderer
	rs.mu.Lock()
	for udn, d := range rs.devices {
		if now.After(d.expires) {
			delete(rs.devices, udn)
			gone = append(gone, d)
			rs.dirty.Store(true)
		}
	}
	rs.mu.Unlock()
	for _, d := range gone {
		go d.gone()
	}
}

// list returns the renderers by name.
func (rs *renderers) list() []*renderer {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	out := slices.Collect(func(yield func(*renderer) bool) {
		for _, d := range rs.devices {
			if !yield(d) {
				return
			}
		}
	})
	slices.SortFunc(out, func(a, b *renderer) int {
		return cmp.Or(strings.Compare(a.name, b.name), strings.Compare(a.udn, b.udn))
	})
	return out
}

// device returns a renderer by UDN.
func (rs *renderers) device(udn string) (*renderer, bool) {
	if rs == nil {
		return nil, false
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	d, ok := rs.devices[udn]
	return d, ok
}

// stream returns the stream a renderer plays by its playback ID, with the
// renderer's profile.
func (rs *renderers) stream(playback string) (*stream, *profile.Profile, bool) {
	if rs == nil {
		return nil, nil, false
	}
	for _, d := range rs.list() {
		d.mu.Lock()
		s := d.cur
		d.mu.Unlock()
		if s != nil && s.resp.GetPlaybackId() == playback {
			return s, d.profile, true
		}
	}
	return nil, nil, false
}

// publish lists the renderers as the server's remote devices.
func (rs *renderers) publish(ctx context.Context) {
	rs.dirty.Store(false)
	var list []*pluginv1.Device
	for _, d := range rs.list() {
		commands := []pluginv1.CommandKind{
			pluginv1.CommandKind_COMMAND_KIND_PLAY, pluginv1.CommandKind_COMMAND_KIND_PLAY_STATE, pluginv1.CommandKind_COMMAND_KIND_SEEK,
		}
		if d.rendering != "" {
			commands = append(commands, pluginv1.CommandKind_COMMAND_KIND_VOLUME)
		}
		list = append(list, pluginv1.Device_builder{
			Id: proto.String(d.udn), Name: proto.String(d.name), Product: proto.String(d.product), Commands: commands,
		}.Build())
	}
	if err := setDevices(ctx, list); err != nil {
		rs.dirty.Store(true)
		if ctx.Err() == nil {
			rs.p.log.WarnContext(ctx, "list the renderers", "err", err)
		}
	}
}

func setDevices(ctx context.Context, list []*pluginv1.Device) error {
	host := pluginv1connect.NewHostServiceClient(guest.HostClient(), guest.HostURL)
	_, err := host.SetDevices(ctx, pluginv1.SetDevicesRequest_builder{Devices: list}.Build())
	return err
}

// close stops what the renderers play and lists them no longer.
func (rs *renderers) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, d := range rs.list() {
		d.mu.Lock()
		d.end(ctx, d.position)
		d.mu.Unlock()
	}
	_ = setDevices(ctx, nil)
}

// SendCommand carries out a command on a renderer.
func (p *plugin) SendCommand(ctx context.Context, req *pluginv1.SendCommandRequest) (*pluginv1.SendCommandResponse, error) {
	p.mu.Lock()
	rs := p.renderers
	p.mu.Unlock()
	d, ok := rs.device(req.GetDeviceId())
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no renderer %q", req.GetDeviceId()))
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	cmd := req.GetCommand()
	var err error
	switch {
	case cmd.HasPlay():
		play := cmd.GetPlay()
		if len(play.GetItemIds()) == 0 || int(play.GetStartIndex()) >= len(play.GetItemIds()) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no item to play"))
		}
		d.end(ctx, d.position)
		d.queue, d.index = play.GetItemIds(), int(play.GetStartIndex())
		err = d.start(ctx, req.GetUserName(), play.GetStartPosition().AsDuration())
	case cmd.HasPlayState():
		err = d.playState(ctx, cmd.GetPlayState().GetCommand())
	case cmd.HasSeek():
		err = d.seek(ctx, cmd.GetSeek().GetPosition().AsDuration())
	case cmd.HasVolume():
		err = d.volume(ctx, cmd.GetVolume())
	default:
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("DLNA renderers take no such command"))
	}
	if err != nil {
		if _, ok := errors.AsType[*connect.Error](err); !ok {
			err = connect.NewError(connect.CodeUnavailable, err)
		}
		return nil, err
	}
	return &pluginv1.SendCommandResponse{}, nil
}

// call invokes an action of a renderer's service on instance 0.
func (d *renderer) call(ctx context.Context, controlURL, serviceType, action string, args ...upnp.Arg) (map[string]string, error) {
	args = append([]upnp.Arg{{Name: "InstanceID", Value: "0"}}, args...)
	out, err := upnp.Call(ctx, d.r.client, controlURL, serviceType, action, args)
	if err != nil {
		return nil, fmt.Errorf("%s on %s: %w", action, d.name, err)
	}
	return out, nil
}

func (d *renderer) transportCall(ctx context.Context, action string, args ...upnp.Arg) (map[string]string, error) {
	return d.call(ctx, d.transport, avTransport, action, args...)
}

// start plays the queue's current item from a position, acting as a user.
func (d *renderer) start(ctx context.Context, user string, position time.Duration) error {
	p := d.r.p
	s, err := p.startStream(ctx, user, d.udn, d.queue[d.index], d.profile, position)
	if err != nil {
		return err
	}
	_, _, info, _ := p.state()
	root := p.root(d.local, info)
	uri := root + "/play/" + s.resp.GetPlaybackId() + "/stream." + s.ext
	meta := didl.Marshal([]didl.Object{d.metadata(s, root, uri)})
	_, _ = d.transportCall(ctx, "Stop")
	_, err = d.transportCall(ctx, "SetAVTransportURI", upnp.Arg{Name: "CurrentURI", Value: uri}, upnp.Arg{Name: "CurrentURIMetaData", Value: meta})
	if err == nil {
		_, err = d.transportCall(ctx, "Play", upnp.Arg{Name: "Speed", Value: "1"})
	}
	if err != nil {
		_ = s.stop(ctx, position)
		return err
	}
	d.cur, d.paused, d.playing, d.failures = s, false, false, 0
	d.started, d.reported = time.Now(), time.Time{}
	d.offset, d.position = 0, s.start()
	if !s.direct {
		// Transcodes begin where the playback starts.
		d.offset = s.start()
	} else if position > 0 {
		// Renderers seek in files they read whole once they play them.
		_, _ = d.transportCall(ctx, "Seek", upnp.Arg{Name: "Unit", Value: "REL_TIME"}, upnp.Arg{Name: "Target", Value: clock(position)})
		d.position = position
	}
	return nil
}

// metadata describes a stream as SetAVTransportURI passes it.
func (d *renderer) metadata(s *stream, root, uri string) didl.Object {
	o := didl.Object{
		ID: s.item.GetId(), ParentID: "-1", Title: s.item.GetName(), Class: classes[s.item.GetKind()], ChildCount: -1,
		Artists: s.item.GetArtists(), Album: s.item.GetAlbumName(), Genres: s.item.GetGenres(),
		Res: []didl.Res{{URL: uri, ProtocolInfo: "http-get:*:" + s.mime + ":" + contentFeatures(s.direct, false), Duration: s.duration}},
	}
	if img := primaryImage(s.item); img != nil {
		o.AlbumArt, o.AlbumArtProfile = root+"/images/"+img.GetId()+"?size=160", "JPEG_TN"
	}
	return o
}

// end stops the playback in progress at a position, a negative one when
// it played to the end.
func (d *renderer) end(ctx context.Context, position time.Duration) {
	if d.cur == nil {
		return
	}
	if err := d.cur.stop(ctx, position); err != nil {
		d.r.p.log.DebugContext(ctx, "stop playback", "renderer", d.name, "err", err)
	}
	d.cur = nil
}

// gone ends what a renderer no longer listed plays.
func (d *renderer) gone() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.end(ctx, d.position)
	d.queue = nil
}

func (d *renderer) playState(ctx context.Context, c pluginv1.PlayStateCommand) error {
	if d.cur == nil && c != pluginv1.PlayStateCommand_PLAY_STATE_COMMAND_STOP {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s plays nothing", d.name))
	}
	switch c {
	case pluginv1.PlayStateCommand_PLAY_STATE_COMMAND_PAUSE:
		if _, err := d.transportCall(ctx, "Pause"); err != nil {
			return err
		}
		d.paused = true
		return d.report(ctx, true)
	case pluginv1.PlayStateCommand_PLAY_STATE_COMMAND_UNPAUSE:
		if _, err := d.transportCall(ctx, "Play", upnp.Arg{Name: "Speed", Value: "1"}); err != nil {
			return err
		}
		d.paused = false
		return d.report(ctx, true)
	case pluginv1.PlayStateCommand_PLAY_STATE_COMMAND_STOP:
		_, err := d.transportCall(ctx, "Stop")
		d.end(ctx, d.position)
		d.queue = nil
		return err
	case pluginv1.PlayStateCommand_PLAY_STATE_COMMAND_NEXT, pluginv1.PlayStateCommand_PLAY_STATE_COMMAND_PREVIOUS:
		i := d.index + 1
		if c == pluginv1.PlayStateCommand_PLAY_STATE_COMMAND_PREVIOUS {
			i = d.index - 1
		}
		if i < 0 || i >= len(d.queue) {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("no such item in the queue"))
		}
		user := d.cur.user
		d.end(ctx, d.position)
		d.index = i
		return d.start(ctx, user, 0)
	}
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("play state command %v", c))
}

// seek moves the playback: renderers seek in files, transcodes start
// again there.
func (d *renderer) seek(ctx context.Context, position time.Duration) error {
	if d.cur == nil {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s plays nothing", d.name))
	}
	if d.cur.direct {
		_, err := d.transportCall(ctx, "Seek", upnp.Arg{Name: "Unit", Value: "REL_TIME"}, upnp.Arg{Name: "Target", Value: clock(position)})
		if err == nil {
			d.position = position
			err = d.report(ctx, true)
		}
		return err
	}
	user := d.cur.user
	d.end(ctx, position)
	return d.start(ctx, user, position)
}

func (d *renderer) volume(ctx context.Context, v *pluginv1.Volume) error {
	if d.rendering == "" {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s has no volume control", d.name))
	}
	master := upnp.Arg{Name: "Channel", Value: "Master"}
	if v.HasLevel() {
		_, err := d.call(ctx, d.rendering, renderingControl, "SetVolume", master, upnp.Arg{Name: "DesiredVolume", Value: strconv.Itoa(int(v.GetLevel()))})
		if err != nil {
			return err
		}
	}
	if v.HasMuted() {
		mute := "0"
		if v.GetMuted() {
			mute = "1"
		}
		if _, err := d.call(ctx, d.rendering, renderingControl, "SetMute", master, upnp.Arg{Name: "DesiredMute", Value: mute}); err != nil {
			return err
		}
	}
	return nil
}

// report tells the server where the playback is, at most every
// reportInterval unless now is set.
func (d *renderer) report(ctx context.Context, now bool) error {
	if d.cur == nil || !now && time.Since(d.reported) < reportInterval {
		return nil
	}
	d.reported = time.Now()
	return d.cur.report(ctx, d.position, d.paused)
}

// poll asks the renderer where it is and reports it, moving on to the
// next item of the queue at the end of one.
func (d *renderer) poll(ctx context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cur == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	info, err := d.transportCall(ctx, "GetTransportInfo")
	var pos map[string]string
	if err == nil {
		pos, err = d.transportCall(ctx, "GetPositionInfo")
	}
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		if d.failures++; d.failures >= maxFailures {
			d.r.p.log.InfoContext(ctx, "renderer unreachable; playback stopped", "renderer", d.name, "err", err)
			d.end(ctx, d.position)
		}
		return
	}
	d.failures = 0
	if t, err := didl.ParseDuration(pos["RelTime"]); err == nil && (t > 0 || d.playing) {
		d.position = d.offset + t
	}
	ours := pos["TrackURI"] == "" || strings.Contains(pos["TrackURI"], "/play/"+d.cur.resp.GetPlaybackId()+"/")
	switch state := info["CurrentTransportState"]; {
	case !ours && d.playing:
		// Something else plays on the renderer now.
		d.end(ctx, d.position)
		d.queue = nil
	case state == "PLAYING":
		d.playing, d.paused = true, false
		_ = d.report(ctx, false)
	case state == "PAUSED_PLAYBACK":
		changed := !d.paused
		d.paused = true
		_ = d.report(ctx, changed)
	case state == "STOPPED" || state == "NO_MEDIA_PRESENT":
		if !d.playing && time.Since(d.started) < startGrace {
			return
		}
		if d.cur.duration > 0 && d.position < d.cur.duration-endSlack {
			// Stopped on the renderer.
			d.end(ctx, d.position)
			d.queue = nil
			return
		}
		user := d.cur.user
		d.end(ctx, -1)
		if d.index+1 < len(d.queue) {
			d.index++
			if err := d.start(ctx, user, 0); err != nil {
				d.r.p.log.WarnContext(ctx, "play the next item", "renderer", d.name, "err", err)
			}
		}
	}
}

// clock writes a position as AVTransport's Seek takes it, H:MM:SS.
func clock(d time.Duration) string {
	s := int(d / time.Second)
	return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
}
