package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/guest"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
	"github.com/mavioai/mavio/plugins/dlna/internal/didl"
	"github.com/mavioai/mavio/plugins/dlna/internal/profile"
	"github.com/mavioai/mavio/plugins/dlna/internal/upnp"
)

// Object IDs of the content directory besides items, which go by their
// IDs.
const (
	rootID        = "0"
	libraryPrefix = "library:"
)

// maxPage caps the objects of a Browse or Search response; control points
// page through the rest.
const maxPage = 200

// browsable are the kinds the content directory shows: all but books,
// which renderers do not play.
var browsable = func() []libraryv1.ItemKind {
	var out []libraryv1.ItemKind
	for k := range libraryv1.ItemKind_name {
		kind := libraryv1.ItemKind(k)
		if kind != libraryv1.ItemKind_ITEM_KIND_UNSPECIFIED && kind != libraryv1.ItemKind_ITEM_KIND_BOOK {
			out = append(out, kind)
		}
	}
	slices.Sort(out)
	return out
}()

// classes are the UPnP classes of item kinds.
var classes = map[libraryv1.ItemKind]string{
	libraryv1.ItemKind_ITEM_KIND_MOVIE:        didl.ClassMovie,
	libraryv1.ItemKind_ITEM_KIND_EPISODE:      didl.ClassVideo,
	libraryv1.ItemKind_ITEM_KIND_VIDEO:        didl.ClassVideo,
	libraryv1.ItemKind_ITEM_KIND_MUSIC_VIDEO:  didl.ClassMusicVideo,
	libraryv1.ItemKind_ITEM_KIND_TRACK:        didl.ClassMusicTrack,
	libraryv1.ItemKind_ITEM_KIND_AUDIOBOOK:    didl.ClassAudioBook,
	libraryv1.ItemKind_ITEM_KIND_PHOTO:        didl.ClassPhoto,
	libraryv1.ItemKind_ITEM_KIND_MUSIC_ALBUM:  didl.ClassMusicAlbum,
	libraryv1.ItemKind_ITEM_KIND_PHOTO_ALBUM:  didl.ClassPhotoAlbum,
	libraryv1.ItemKind_ITEM_KIND_MUSIC_ARTIST: didl.ClassMusicArtist,
	libraryv1.ItemKind_ITEM_KIND_PLAYLIST:     didl.ClassPlaylist,
	libraryv1.ItemKind_ITEM_KIND_SERIES:       didl.ClassStorageFolder,
	libraryv1.ItemKind_ITEM_KIND_SEASON:       didl.ClassStorageFolder,
	libraryv1.ItemKind_ITEM_KIND_FOLDER:       didl.ClassStorageFolder,
	libraryv1.ItemKind_ITEM_KIND_COLLECTION:   didl.ClassStorageFolder,
}

func isContainer(k libraryv1.ItemKind) bool {
	return strings.HasPrefix(classes[k], "object.container")
}

// mediaKind returns whether an item is played as video, as audio or shown
// as an image, or "" when it is none of them.
func mediaKind(k libraryv1.ItemKind) string {
	switch c := classes[k]; {
	case strings.HasPrefix(c, didl.ClassVideo):
		return "video"
	case strings.HasPrefix(c, "object.item.audioItem"):
		return "audio"
	case c == didl.ClassPhoto:
		return "image"
	}
	return ""
}

func mediaKindName(k playbackv1.MediaKind) string {
	switch k {
	case playbackv1.MediaKind_MEDIA_KIND_VIDEO:
		return "video"
	case playbackv1.MediaKind_MEDIA_KIND_AUDIO:
		return "audio"
	}
	return "image"
}

// browser answers ContentDirectory requests of one control point.
type browser struct {
	p    *plugin
	user string
	// root is the URL of the plugin's routes as the control point reaches
	// them.
	root    string
	profile *profile.Profile
	items   libraryv1connect.ItemServiceClient
	libs    libraryv1connect.LibraryServiceClient
	name    string
}

func (p *plugin) newBrowser(r *http.Request) *browser {
	c, set, info, _ := p.state()
	client := hostClient(c.User, "")
	return &browser{
		p: p, user: c.User, root: p.requestRoot(r), profile: set.Match(profile.Device{Header: r.Header}),
		items: libraryv1connect.NewItemServiceClient(client, guest.HostURL),
		libs:  libraryv1connect.NewLibraryServiceClient(client, guest.HostURL),
		name:  p.friendlyName(c, info),
	}
}

func (p *plugin) contentDirectoryAction(r *http.Request, a upnp.Action) ([]upnp.Arg, *upnp.Fault) {
	updateID := strconv.FormatUint(uint64(p.updateID.Load()), 10)
	switch a.Name {
	case "GetSearchCapabilities":
		return []upnp.Arg{{Name: "SearchCaps", Value: "dc:title,upnp:class"}}, nil
	case "GetSortCapabilities":
		return []upnp.Arg{{Name: "SortCaps", Value: "dc:title,dc:date,upnp:originalTrackNumber"}}, nil
	case "GetSystemUpdateID":
		return []upnp.Arg{{Name: "Id", Value: updateID}}, nil
	case "X_GetFeatureList":
		return []upnp.Arg{{Name: "FeatureList", Value: featureList}}, nil
	case "Browse", "Search":
	default:
		return nil, upnp.ErrInvalidAction
	}
	start, err1 := strconv.ParseUint(a.Args["StartingIndex"], 10, 32)
	count, err2 := strconv.ParseUint(a.Args["RequestedCount"], 10, 32)
	if err1 != nil || err2 != nil {
		return nil, upnp.ErrInvalidArgs
	}
	if count == 0 || count > maxPage {
		count = maxPage
	}
	sort, ok := parseSort(a.Args["SortCriteria"])
	if !ok {
		return nil, upnp.ErrBadSort
	}
	b := p.newBrowser(r)
	ctx := r.Context()
	var objects []didl.Object
	var total int
	var err error
	switch {
	case a.Name == "Search":
		q, ok := parseSearch(a.Args["SearchCriteria"])
		if !ok {
			return nil, upnp.ErrBadSearch
		}
		objects, total, err = b.search(ctx, a.Args["ContainerID"], q, sort, int(start), int(count))
	case a.Args["BrowseFlag"] == "BrowseMetadata":
		var o didl.Object
		o, err = b.metadata(ctx, a.Args["ObjectID"])
		objects, total = []didl.Object{o}, 1
	case a.Args["BrowseFlag"] == "BrowseDirectChildren":
		objects, total, err = b.children(ctx, a.Args["ObjectID"], sort, int(start), int(count))
	default:
		return nil, upnp.ErrInvalidArgs
	}
	switch {
	case connect.CodeOf(err) == connect.CodeNotFound || connect.CodeOf(err) == connect.CodeInvalidArgument || errors.Is(err, errNoObject):
		return nil, upnp.ErrNoSuchObject
	case err != nil:
		p.log.WarnContext(ctx, "content directory", "action", a.Name, "err", err)
		return nil, upnp.ErrCannotProcess
	}
	return []upnp.Arg{
		{Name: "Result", Value: didl.Marshal(objects)},
		{Name: "NumberReturned", Value: strconv.Itoa(len(objects))},
		{Name: "TotalMatches", Value: strconv.Itoa(total)},
		{Name: "UpdateID", Value: updateID},
	}, nil
}

var errNoObject = errors.New("no such object")

// featureList answers Samsung's X_GetFeatureList: every kind of media
// starts at the root.
const featureList = `<?xml version="1.0" encoding="utf-8"?>` +
	`<Features xmlns="urn:schemas-upnp-org:av:avs" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" ` +
	`xsi:schemaLocation="urn:schemas-upnp-org:av:avs http://www.upnp.org/schemas/av/avs.xsd">` +
	`<Feature name="samsung.com_BASICVIEW" version="1">` +
	`<container id="0" type="object.item.imageItem"/><container id="0" type="object.item.audioItem"/>` +
	`<container id="0" type="object.item.videoItem"/></Feature></Features>`

// metadata describes one object.
func (b *browser) metadata(ctx context.Context, id string) (didl.Object, error) {
	switch {
	case id == rootID:
		libs, err := b.libraries(ctx)
		if err != nil {
			return didl.Object{}, err
		}
		return didl.Object{
			ID: rootID, ParentID: "-1", Container: true, Searchable: true, ChildCount: len(libs),
			Title: b.name, Class: didl.ClassStorageFolder,
		}, nil
	case strings.HasPrefix(id, libraryPrefix):
		resp, err := b.libs.GetLibrary(ctx, libraryv1.GetLibraryRequest_builder{Id: proto.String(strings.TrimPrefix(id, libraryPrefix))}.Build())
		if err != nil {
			return didl.Object{}, err
		}
		o := libraryObject(resp.GetLibrary())
		o.ChildCount, err = b.count(ctx, libraryv1.ListItemsRequest_builder{LibraryIds: []string{resp.GetLibrary().GetId()}, TopLevel: proto.Bool(true)})
		return o, err
	}
	resp, err := b.items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: proto.String(id)}.Build())
	if err != nil {
		return didl.Object{}, err
	}
	it := resp.GetItem()
	if _, ok := classes[it.GetKind()]; !ok {
		return didl.Object{}, errNoObject
	}
	parent := it.GetParentId()
	if parent == "" {
		parent = libraryPrefix + it.GetLibraryId()
	}
	objs, err := b.objects(ctx, []*libraryv1.Item{it}, parent, map[string][]*libraryv1.MediaSource{it.GetId(): resp.GetMediaSources()})
	if err != nil {
		return didl.Object{}, err
	}
	return objs[0], nil
}

// libraries lists the user's libraries the content directory shows.
func (b *browser) libraries(ctx context.Context) ([]*libraryv1.Library, error) {
	resp, err := b.libs.ListLibraries(ctx, &libraryv1.ListLibrariesRequest{})
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(resp.GetLibraries(), func(l *libraryv1.Library) bool {
		return l.GetKind() == libraryv1.LibraryKind_LIBRARY_KIND_BOOKS
	}), nil
}

func libraryObject(l *libraryv1.Library) didl.Object {
	return didl.Object{
		ID: libraryPrefix + l.GetId(), ParentID: rootID, Container: true, Searchable: true,
		Title: l.GetName(), Class: didl.ClassStorageFolder,
	}
}

// children lists a container's children.
func (b *browser) children(ctx context.Context, id string, sort []*libraryv1.SortSpec, start, count int) ([]didl.Object, int, error) {
	req := libraryv1.ListItemsRequest_builder{Kinds: browsable, Sort: sort, Offset: proto.Int32(int32(start)), Limit: proto.Int32(int32(count))}
	switch {
	case id == rootID:
		libs, err := b.libraries(ctx)
		if err != nil {
			return nil, 0, err
		}
		page := libs[min(start, len(libs)):min(start+count, len(libs))]
		out := make([]didl.Object, len(page))
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(8)
		for i, l := range page {
			out[i] = libraryObject(l)
			g.Go(func() error {
				n, err := b.count(gctx, libraryv1.ListItemsRequest_builder{LibraryIds: []string{l.GetId()}, TopLevel: proto.Bool(true)})
				out[i].ChildCount = n
				return err
			})
		}
		return out, len(libs), g.Wait()
	case strings.HasPrefix(id, libraryPrefix):
		req.LibraryIds, req.TopLevel = []string{strings.TrimPrefix(id, libraryPrefix)}, proto.Bool(true)
		if len(sort) == 0 {
			req.Sort = byName
		}
	default:
		parent, err := b.items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: proto.String(id)}.Build())
		if err != nil {
			return nil, 0, err
		}
		if !isContainer(parent.GetItem().GetKind()) {
			return nil, 0, nil
		}
		req.ParentId = proto.String(id)
		if len(sort) == 0 {
			req.Sort = defaultSort(parent.GetItem().GetKind())
		}
	}
	return b.list(ctx, req, id)
}

var byName = []*libraryv1.SortSpec{libraryv1.SortSpec_builder{Field: libraryv1.SortField_SORT_FIELD_NAME.Enum()}.Build()}

func sortBy(fields ...libraryv1.SortField) []*libraryv1.SortSpec {
	var out []*libraryv1.SortSpec
	for _, f := range fields {
		out = append(out, libraryv1.SortSpec_builder{Field: f.Enum()}.Build())
	}
	return out
}

// defaultSort orders the children of a container: by number within
// series, seasons and albums, by year within artists, in their own order
// within collections and playlists, else by name.
func defaultSort(parent libraryv1.ItemKind) []*libraryv1.SortSpec {
	switch parent {
	case libraryv1.ItemKind_ITEM_KIND_SERIES, libraryv1.ItemKind_ITEM_KIND_SEASON, libraryv1.ItemKind_ITEM_KIND_MUSIC_ALBUM:
		return sortBy(libraryv1.SortField_SORT_FIELD_INDEX, libraryv1.SortField_SORT_FIELD_NAME)
	case libraryv1.ItemKind_ITEM_KIND_MUSIC_ARTIST:
		return sortBy(libraryv1.SortField_SORT_FIELD_PRODUCTION_YEAR, libraryv1.SortField_SORT_FIELD_NAME)
	case libraryv1.ItemKind_ITEM_KIND_COLLECTION, libraryv1.ItemKind_ITEM_KIND_PLAYLIST:
		return nil
	}
	return byName
}

// list lists a page of items as objects under parent; an empty parent
// gives each object its own.
func (b *browser) list(ctx context.Context, req libraryv1.ListItemsRequest_builder, parent string) ([]didl.Object, int, error) {
	resp, err := b.items.ListItems(ctx, req.Build())
	if err != nil {
		return nil, 0, err
	}
	objs, err := b.objects(ctx, resp.GetItems(), parent, nil)
	return objs, int(resp.GetTotal()), err
}

// count returns how many items a request matches.
func (b *browser) count(ctx context.Context, req libraryv1.ListItemsRequest_builder) (int, error) {
	req.Kinds, req.Limit = browsable, proto.Int32(1)
	resp, err := b.items.ListItems(ctx, req.Build())
	return int(resp.GetTotal()), err
}

// objects describes items: containers with their child counts, playable
// items with their media sources, looked up unless sources holds them.
func (b *browser) objects(ctx context.Context, items []*libraryv1.Item, parent string, sources map[string][]*libraryv1.MediaSource) ([]didl.Object, error) {
	out := make([]didl.Object, len(items))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(8)
	for i, it := range items {
		pid := parent
		if pid == "" {
			if pid = it.GetParentId(); pid == "" {
				pid = libraryPrefix + it.GetLibraryId()
			}
		}
		out[i] = b.object(it, pid, nil)
		switch {
		case isContainer(it.GetKind()):
			g.Go(func() error {
				n, err := b.count(gctx, libraryv1.ListItemsRequest_builder{ParentId: proto.String(it.GetId())})
				out[i].ChildCount = n
				return err
			})
		case mediaKind(it.GetKind()) == "video" || mediaKind(it.GetKind()) == "audio":
			if ms, ok := sources[it.GetId()]; ok {
				out[i] = b.object(it, pid, ms)
				continue
			}
			g.Go(func() error {
				resp, err := b.items.GetItem(gctx, libraryv1.GetItemRequest_builder{Id: proto.String(it.GetId())}.Build())
				if connect.CodeOf(err) == connect.CodeNotFound {
					return nil
				} else if err != nil {
					return err
				}
				out[i] = b.object(it, pid, resp.GetMediaSources())
				return nil
			})
		}
	}
	return out, g.Wait()
}

// object describes an item.
func (b *browser) object(it *libraryv1.Item, parent string, sources []*libraryv1.MediaSource) didl.Object {
	o := didl.Object{
		ID: it.GetId(), ParentID: parent, Title: it.GetName(), Class: classes[it.GetKind()],
		Container: isContainer(it.GetKind()), Searchable: isContainer(it.GetKind()), ChildCount: -1,
		Description: it.GetOverview(), Genres: it.GetGenres(), Artists: it.GetArtists(), Album: it.GetAlbumName(),
	}
	if d := it.GetPremiereDate(); d != nil {
		o.Date = d.AsTime()
	}
	if aa := it.GetAlbumArtists(); len(aa) > 0 {
		o.AlbumArtist = aa[0]
	}
	switch it.GetKind() {
	case libraryv1.ItemKind_ITEM_KIND_TRACK, libraryv1.ItemKind_ITEM_KIND_EPISODE:
		o.TrackNumber = int(it.GetIndexNumber())
	}
	primary := primaryImage(it)
	if primary != nil {
		o.AlbumArt, o.AlbumArtProfile = b.imageURL(primary, 160), "JPEG_TN"
	}
	switch kind := mediaKind(it.GetKind()); {
	case kind == "image" && primary != nil:
		res := didl.Res{URL: b.imageURL(primary, 4096), ProtocolInfo: imageProtocolInfo("JPEG_LRG")}
		if w, h := primary.GetWidth(), primary.GetHeight(); w > 0 && h > 0 && w <= 4096 && h <= 4096 {
			res.Resolution = strconv.Itoa(int(w)) + "x" + strconv.Itoa(int(h))
		}
		o.Res = append(o.Res, res, didl.Res{URL: b.imageURL(primary, 160), ProtocolInfo: imageProtocolInfo("JPEG_TN")})
	case (kind == "video" || kind == "audio") && len(sources) > 0:
		o.Res = append(o.Res, mediaRes(b.root, b.profile, it.GetId(), kind, sources[0]))
		if kind == "video" && hasTextSubtitles(sources[0]) {
			sub := b.root + "/subtitles/" + it.GetId() + "/subtitles.srt?profile=" + url.QueryEscape(b.profile.Name)
			if b.profile.SubtitleRes {
				o.Res = append(o.Res, didl.Res{URL: sub, ProtocolInfo: "http-get:*:text/srt:*"})
			}
			if b.profile.CaptionInfo {
				o.Caption, o.CaptionType = sub, "srt"
			}
		}
	}
	return o
}

func primaryImage(it *libraryv1.Item) *libraryv1.Image {
	for _, img := range it.GetImages() {
		if img.GetKind() == libraryv1.ImageKind_IMAGE_KIND_PRIMARY {
			return img
		}
	}
	return nil
}

// imageURL returns the URL of an image in JPEG, at most size pixels wide
// and high.
func (b *browser) imageURL(img *libraryv1.Image, size int) string {
	return b.root + "/images/" + img.GetId() + "?size=" + strconv.Itoa(size)
}

func hasTextSubtitles(ms *libraryv1.MediaSource) bool {
	return slices.ContainsFunc(ms.GetStreams(), func(s *libraryv1.MediaStream) bool {
		return s.GetKind() == libraryv1.StreamKind_STREAM_KIND_SUBTITLE && s.GetTextBased()
	})
}

// DLNA.ORG_FLAGS: streaming transfer, background transfer, connection
// stall and DLNA 1.5 for media; interactive transfer instead of streaming
// for images.
const (
	streamingFlags   = "01700000000000000000000000000000"
	interactiveFlags = "00d00000000000000000000000000000"
)

func imageProtocolInfo(pn string) string { return "http-get:*:image/jpeg:" + imageFeatures(pn) }

// imageFeatures returns the DLNA fourth field of protocol info for JPEG
// images of a profile, JPEG_TN or JPEG_LRG.
func imageFeatures(pn string) string {
	return "DLNA.ORG_PN=" + pn + ";DLNA.ORG_OP=00;DLNA.ORG_CI=1;DLNA.ORG_FLAGS=" + interactiveFlags
}

// contentFeatures returns the DLNA fourth field of protocol info, which
// contentFeatures.dlna.org repeats: direct play seeks by bytes, and
// transcodes by time when the profile does.
func contentFeatures(direct, timeSeek bool) string {
	op, ci := "00", "1"
	switch {
	case direct:
		op, ci = "01", "0"
	case timeSeek:
		op = "10"
	}
	return "DLNA.ORG_OP=" + op + ";DLNA.ORG_CI=" + ci + ";DLNA.ORG_FLAGS=" + streamingFlags
}

// streamFormat is how a media source reaches a profile: as it is, or
// transcoded into the profile's container for its kind.
type streamFormat struct {
	direct    bool
	container string
	mime      string
}

// formatFor works out a source's format for a profile, as the server's
// decision would for its direct play profiles, without their conditions.
func formatFor(p *profile.Profile, kind string, ms *libraryv1.MediaSource) streamFormat {
	caps := p.ClientCapabilities()
	var video, audio string
	for _, s := range ms.GetStreams() {
		switch {
		case s.GetKind() == libraryv1.StreamKind_STREAM_KIND_VIDEO && video == "":
			video = s.GetCodec()
		case s.GetKind() == libraryv1.StreamKind_STREAM_KIND_AUDIO && (audio == "" || s.GetDefault()):
			audio = s.GetCodec()
		}
	}
	for _, d := range caps.GetDirectPlay() {
		if mediaKindName(d.GetKind()) != kind || !profile.ContainsName(d.GetContainer(), ms.GetContainer()) {
			continue
		}
		if kind == "video" && video != "" && !profile.ContainsName(d.GetVideoCodec(), video) {
			continue
		}
		if audio != "" && !profile.ContainsName(d.GetAudioCodec(), audio) {
			continue
		}
		return streamFormat{direct: true, container: ms.GetContainer(), mime: p.MimeType(ms.GetContainer(), kind)}
	}
	for _, t := range caps.GetTranscoding() {
		if mediaKindName(t.GetKind()) == kind && t.GetProtocol() != playbackv1.Protocol_PROTOCOL_HLS {
			return streamFormat{container: t.GetContainer(), mime: p.MimeType(t.GetContainer(), kind)}
		}
	}
	return streamFormat{direct: true, container: ms.GetContainer(), mime: p.MimeType(ms.GetContainer(), kind)}
}

// extension returns the file extension of a container.
func extension(container string) string {
	first, _, _ := strings.Cut(strings.ToLower(container), ",")
	switch {
	case first == "mov" && strings.Contains(container, "mp4"):
		return "mp4"
	case first == "":
		return "bin"
	}
	return first
}

// mediaRes describes the media of an item as a profile receives it.
func mediaRes(root string, p *profile.Profile, itemID, kind string, ms *libraryv1.MediaSource) didl.Res {
	f := formatFor(p, kind, ms)
	r := didl.Res{
		URL:          root + "/media/" + itemID + "/stream." + extension(f.container) + "?profile=" + url.QueryEscape(p.Name),
		ProtocolInfo: "http-get:*:" + f.mime + ":" + contentFeatures(f.direct, p.TimeSeek),
		Duration:     ms.GetDuration().AsDuration(),
	}
	if f.direct {
		r.Size, r.Bitrate = ms.GetSizeBytes(), ms.GetBitrate()/8
	}
	for _, s := range ms.GetStreams() {
		switch {
		case s.GetKind() == libraryv1.StreamKind_STREAM_KIND_VIDEO && r.Resolution == "" && s.GetWidth() > 0:
			r.Resolution = strconv.Itoa(int(s.GetWidth())) + "x" + strconv.Itoa(int(s.GetHeight()))
		case s.GetKind() == libraryv1.StreamKind_STREAM_KIND_AUDIO && r.AudioChannels == 0:
			r.AudioChannels, r.SampleFrequency = int(s.GetChannels()), int(s.GetSampleRate())
		}
	}
	return r
}

// parseSort reads SortCriteria, "+dc:title,-dc:date"; properties it does
// not know are left out.
func parseSort(s string) ([]*libraryv1.SortSpec, bool) {
	var out []*libraryv1.SortSpec
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		desc := strings.HasPrefix(part, "-")
		if !desc && !strings.HasPrefix(part, "+") {
			return nil, false
		}
		var f libraryv1.SortField
		switch part[1:] {
		case "dc:title":
			f = libraryv1.SortField_SORT_FIELD_NAME
		case "dc:date":
			f = libraryv1.SortField_SORT_FIELD_PREMIERE_DATE
		case "upnp:originalTrackNumber":
			f = libraryv1.SortField_SORT_FIELD_INDEX
		default:
			continue
		}
		out = append(out, libraryv1.SortSpec_builder{Field: f.Enum(), Descending: proto.Bool(desc)}.Build())
	}
	return out, true
}

// searchQuery is what a search asks for: kinds, none meaning any, and
// text in titles.
type searchQuery struct {
	kinds []libraryv1.ItemKind
	text  string
}

var (
	classCriterion = regexp.MustCompile(`(?i)upnp:class\s+(derivedfrom|=)\s+"([^"]*)"`)
	textCriterion  = regexp.MustCompile(`(?i)(?:dc:title|upnp:artist|upnp:album|dc:creator)\s+(?:contains|=)\s+"((?:[^"\\]|\\.)*)"`)
	anyCriterion   = regexp.MustCompile(`(?i)^[\s()]*(\*)?[\s()]*$`)
)

// parseSearch reads SearchCriteria the way control points write it: class
// criteria joined by "or", and one text criterion. Kinds whose classes
// derive from or equal a class asked for match.
func parseSearch(s string) (searchQuery, bool) {
	var q searchQuery
	if anyCriterion.MatchString(s) {
		return q, true
	}
	classMatches := classCriterion.FindAllStringSubmatch(s, -1)
	textMatches := textCriterion.FindAllStringSubmatch(s, -1)
	if len(classMatches) == 0 && len(textMatches) == 0 {
		return q, false
	}
	for _, m := range classMatches {
		derived := strings.EqualFold(m[1], "derivedfrom")
		for _, k := range browsable {
			c := classes[k]
			if c == m[2] || derived && strings.HasPrefix(c+".", m[2]+".") {
				if !slices.Contains(q.kinds, k) {
					q.kinds = append(q.kinds, k)
				}
			}
		}
	}
	if len(classMatches) > 0 && len(q.kinds) == 0 {
		// Classes nothing has, such as object.item.textItem.
		q.kinds = []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_UNSPECIFIED}
	}
	if len(textMatches) > 0 {
		q.text = strings.ReplaceAll(textMatches[0][1], `\"`, `"`)
		if len(q.text) > 200 {
			q.text = q.text[:200]
		}
	}
	slices.Sort(q.kinds)
	return q, true
}

// search finds the items under a container that match a query.
func (b *browser) search(ctx context.Context, id string, q searchQuery, sort []*libraryv1.SortSpec, start, count int) ([]didl.Object, int, error) {
	if slices.Contains(q.kinds, libraryv1.ItemKind_ITEM_KIND_UNSPECIFIED) {
		return nil, 0, nil
	}
	if len(sort) == 0 {
		sort = byName
	}
	kinds := q.kinds
	if len(kinds) == 0 {
		kinds = browsable
	}
	req := libraryv1.ListItemsRequest_builder{Kinds: kinds, Sort: sort, Offset: proto.Int32(int32(start)), Limit: proto.Int32(int32(count))}
	if q.text != "" {
		req.Search = proto.String(q.text)
	}
	switch {
	case id == rootID:
	case strings.HasPrefix(id, libraryPrefix):
		req.LibraryIds = []string{strings.TrimPrefix(id, libraryPrefix)}
	default:
		parent, err := b.items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: proto.String(id)}.Build())
		if err != nil {
			return nil, 0, err
		}
		k := parent.GetItem().GetKind()
		if !isContainer(k) {
			return nil, 0, nil
		}
		// The members of collections and playlists are linked into them,
		// not their descendants.
		req.ParentId = proto.String(id)
		req.Recursive = proto.Bool(k != libraryv1.ItemKind_ITEM_KIND_COLLECTION && k != libraryv1.ItemKind_ITEM_KIND_PLAYLIST)
	}
	return b.list(ctx, req, "")
}
