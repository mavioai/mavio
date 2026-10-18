package app

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/mavioai/mavio/libs/client-rust/capi"
	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	storagev1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/storage/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/images"
	"github.com/mavioai/mavio/apps/desktop/internal/mpv"
)

// screens is the fixed data of the snapshot tests, libs/design/snapshots/screens.json.
type screens struct {
	Now    time.Time `json:"now"`
	Server struct {
		ID, Name, Version, UserID, UserName string
	} `json:"server"`
	Libraries []struct{ ID, Name, Kind string } `json:"libraries"`
	Items     map[string]struct {
		Kind, Name, Series, Library string
		Year, RuntimeMinutes        int
		Season, Episode             int
	} `json:"items"`
	Home []struct {
		Kind, Library string
		Items         []struct {
			ID              string
			ProgressMinutes int
			Played          bool
		}
	} `json:"home"`
}

func readScreens(t testing.TB) *screens {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "libs", "design", "snapshots", "screens.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s screens
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return &s
}

// fakeCore answers as a server of the fixed data would.
type fakeCore struct {
	s       *screens
	images  string // the artwork server's URL
	servers []*capiv1.ServerRecord
	api     http.Handler // the server's API, which CallServer calls
	// prefs and sessions are the user's on the server's API.
	prefs          *userv1.PlaybackPreferences
	sessions       []*sessionv1.Session
	taskRuns       int                      // the tasks run
	scans          int                      // the libraries scanned
	titleRequests  []*acquisitionv1.Request // the requests of titles
	discovers      []*acquisitionv1.DiscoverRequest
	blocked        []int32 // the collections blocklisted
	users          []*userv1.User
	libraries      []*libraryv1.Library
	invitations    []*userv1.Invitation
	passwords      map[string]string // set by user
	serverSettings *systemv1.ServerSettings
	apiKeys        []*authv1.ApiKey
	proxies        []*systemv1.Proxy
	remotes        []*storagev1.Remote
	offline        []*storagev1.OfflineClient
	answers        []string
	acq            *fakeAcquisition
	wanted         *fakeWanted
	downloads      *fakeDownloads
	subtitles      *fakeSubtitles

	mu       sync.Mutex
	calls    []string
	requests []*capiv1.Request
}

func newFakeCore(s *screens, images string, signedIn bool) *fakeCore {
	f := &fakeCore{s: s, images: images}
	f.api = f.apiHandler()
	if signedIn {
		f.servers = []*capiv1.ServerRecord{f.record()}
	}
	return f
}

func (f *fakeCore) record() *capiv1.ServerRecord {
	return capiv1.ServerRecord_builder{
		Id: f.s.Server.ID, Address: "http://mavio.test", Name: f.s.Server.Name, Version: f.s.Server.Version,
		UserId: f.s.Server.UserID, UserName: f.s.Server.UserName,
	}.Build()
}

func (f *fakeCore) artwork(id string) *capiv1.Artwork {
	return capiv1.Artwork_builder{Url: f.images + "/images/" + id}.Build()
}

func (f *fakeCore) card(id string, progressMinutes int, played bool) *capiv1.Card {
	it := f.s.Items[id]
	kind := map[string]capiv1.CardKind{
		"movie": capiv1.CardKind_CARD_KIND_MOVIE, "series": capiv1.CardKind_CARD_KIND_SERIES, "episode": capiv1.CardKind_CARD_KIND_EPISODE,
	}[it.Kind]
	b := capiv1.Card_builder{
		ServerId: f.s.Server.ID, ItemId: id, Kind: kind, Title: it.Name, Year: int32(it.Year),
		Poster: f.artwork(id + "-poster"), Thumb: f.artwork(id + "-thumb"),
		RuntimeMs: int64(it.RuntimeMinutes) * 60000, Played: played,
	}
	if it.Kind == "episode" {
		b.Subtitle = fmt.Sprintf("%s · S%d:E%d", f.s.Items[it.Series].Name, it.Season, it.Episode)
	}
	if progressMinutes > 0 && it.RuntimeMinutes > 0 {
		b.Progress = new(float32(progressMinutes) / float32(it.RuntimeMinutes))
	}
	return b.Build()
}

func (f *fakeCore) rows() []*capiv1.Row {
	var rows []*capiv1.Row
	for i, h := range f.s.Home {
		r := capiv1.Row_builder{Id: fmt.Sprint("row-", i), ServerId: f.s.Server.ID, ServerName: f.s.Server.Name}
		switch h.Kind {
		case "continueWatching":
			r.Kind = capiv1.RowKind_ROW_KIND_CONTINUE_WATCHING
		case "recentlyAdded":
			r.Kind = capiv1.RowKind_ROW_KIND_RECENTLY_ADDED
		}
		for _, l := range f.s.Libraries {
			if l.ID == h.Library {
				r.LibraryName = l.Name
			}
		}
		var cards []*capiv1.Card
		for _, it := range h.Items {
			cards = append(cards, f.card(it.ID, it.ProgressMinutes, it.Played))
		}
		r.Items = capiv1.Cards_builder{Cards: cards}.Build()
		rows = append(rows, r.Build())
	}
	return rows
}

func (f *fakeCore) Call(ctx context.Context, req *capiv1.Request) (*capiv1.Message, error) {
	return f.Stream(ctx, req, nil)
}

func (f *fakeCore) Stream(ctx context.Context, req *capiv1.Request, part func(*capiv1.Message)) (*capiv1.Message, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req.WhichKind().String())
	f.requests = append(f.requests, req)
	f.mu.Unlock()
	m := capiv1.Message_builder{}
	switch {
	case req.HasListServers():
		m.Servers = capiv1.Servers_builder{Servers: f.servers}.Build()
	case req.HasStart(), req.HasCancelQuickConnect(), req.HasAuthorizeQuickConnect(), req.HasOrderServers(), req.HasSetPlayed(), req.HasSetFavorite(),
		req.HasPollLink(), req.HasCancelLink(), req.HasCancelAccountSignIn():
		m.Done = &capiv1.Done{}
	case req.HasGetAccount():
		m.Account = &capiv1.Account{}
	case req.HasStartLink():
		m.LinkCode = capiv1.LinkCode_builder{
			Request: 1, UserCode: "BDWP-HQPK", VerificationUri: "https://mavio.dev/link",
			VerificationUriComplete: "https://mavio.dev/link?code=BDWP-HQPK",
			ExpiresMs:               time.Now().Add(15 * time.Minute).UnixMilli(), IntervalMs: 5000,
		}.Build()
	case req.HasDiscover():
		m.FoundServers = &capiv1.FoundServers{}
	case req.HasProbe():
		address := req.GetProbe().GetAddress()
		info := capiv1.ServerInfo_builder{Id: f.s.Server.ID, Name: f.s.Server.Name, Address: address}
		if strings.Contains(address, "account") {
			info.Mode, info.MavioDevUrl = capiv1.AccountMode_ACCOUNT_MODE_MAVIO_DEV, "https://mavio.dev/api"
		}
		m.ServerInfo = info.Build()
	case req.HasSignIn():
		if req.GetSignIn().GetPassword() != "secret" {
			return nil, &capi.Error{Code: capiv1.Error_CODE_UNAUTHENTICATED}
		}
		f.servers = []*capiv1.ServerRecord{f.record()}
		m.Server = f.record()
	case req.HasListLibraries():
		var libs []*capiv1.Library
		for _, l := range f.s.Libraries {
			kind := map[string]capiv1.LibraryType{"movies": capiv1.LibraryType_LIBRARY_TYPE_MOVIES, "shows": capiv1.LibraryType_LIBRARY_TYPE_SHOWS}[l.Kind]
			libs = append(libs, capiv1.Library_builder{ServerId: f.s.Server.ID, Id: l.ID, Name: l.Name, Kind: kind}.Build())
		}
		m.Libraries = capiv1.Libraries_builder{Libraries: libs}.Build()
	case req.HasLoadHome():
		if part != nil {
			part(capiv1.Message_builder{Rows: capiv1.Rows_builder{Rows: f.rows()}.Build(), More: true}.Build())
		}
		m.Done = &capiv1.Done{}
	case req.HasGetRecommended():
		m.Rows = capiv1.Rows_builder{Rows: f.rows()[1:]}.Build()
	case req.HasListLibraryItems():
		var cards []*capiv1.Card
		for id, it := range f.s.Items {
			if it.Library == req.GetListLibraryItems().GetLibraryId() && it.Kind != "episode" {
				cards = append(cards, f.card(id, 0, false))
			}
		}
		sortCards(cards)
		m.Page = capiv1.Page_builder{Cards: cards, Total: new(uint32(len(cards)))}.Build()
	case req.HasGetDetails():
		id := req.GetGetDetails().GetItemId()
		if _, ok := f.s.Items[id]; !ok {
			return nil, &capi.Error{Code: capiv1.Error_CODE_NOT_FOUND}
		}
		m.Details = capiv1.Details_builder{
			Card: f.card(id, 0, false), ServerName: f.s.Server.Name,
			Overview: "A made-up title of the snapshot tests, whose overview runs long enough to wrap over a few lines of the details.",
			Genres:   []string{"Drama", "Science Fiction"}, OfficialRating: "PG-13", CommunityRating: new(float32(7.8)),
			Backdrop: f.artwork(id + "-thumb"),
			Sources: []*capiv1.Source{capiv1.Source_builder{
				ServerId: f.s.Server.ID, ItemId: id, MediaSourceId: id + "-source", Name: "1080p", Container: "mkv",
				Audio: []*capiv1.AudioStream{
					capiv1.AudioStream_builder{Index: 1, Language: "eng", Title: "English Stereo", Codec: "aac", Channels: 2}.Build(),
					capiv1.AudioStream_builder{Index: 2, Language: "jpn", Codec: "ac3", Channels: 6}.Build(),
				},
			}.Build()},
		}.Build()
	case req.HasPlay():
		play := req.GetPlay()
		pb := capiv1.Playback_builder{
			Session: uint64(len(f.requests)), Url: "http://mavio.test/videos/" + play.GetItemId(),
			MediaSourceId: play.GetItemId() + "-source", AudioStreamIndex: new(int32(1)), SubtitleStreamIndex: new(int32(3)),
			Subtitles: []*capiv1.SubtitleTrack{
				capiv1.SubtitleTrack_builder{StreamIndex: 3, Language: "eng", Format: "subrip", Default: true}.Build(),
				capiv1.SubtitleTrack_builder{StreamIndex: 4, Language: "jpn", Format: "ass", Forced: true}.Build(),
			},
		}
		if o := play.GetOptions(); o.HasAudioStreamIndex() {
			pb.AudioStreamIndex = new(o.GetAudioStreamIndex())
		}
		if o := play.GetOptions(); o.HasSubtitleStreamIndex() {
			pb.SubtitleStreamIndex = new(o.GetSubtitleStreamIndex())
		}
		if o := play.GetOptions(); o.HasStartPositionMs() {
			pb.StartPositionMs = new(o.GetStartPositionMs())
		}
		m.Playback = pb.Build()
	case req.HasUpdatePlayback(), req.HasStopPlayback():
		m.Done = &capiv1.Done{}
	case req.HasCallServer():
		return f.callServer(ctx, req.GetCallServer())
	case req.HasSearch():
		var cards []*capiv1.Card
		q := strings.ToLower(req.GetSearch().GetQuery())
		for id, it := range f.s.Items {
			if strings.Contains(strings.ToLower(it.Name), q) {
				cards = append(cards, f.card(id, 0, false))
			}
		}
		sortCards(cards)
		m.SearchResults = capiv1.SearchResults_builder{Servers: []*capiv1.ServerResults{
			capiv1.ServerResults_builder{ServerId: f.s.Server.ID, ServerName: f.s.Server.Name, Cards: cards}.Build(),
		}}.Build()
	default:
		return nil, &capi.Error{Code: capiv1.Error_CODE_INVALID, Reason: "the fake core does not answer " + req.WhichKind().String()}
	}
	return m.Build(), nil
}

func sortCards(cards []*capiv1.Card) {
	for i := range cards {
		for j := i + 1; j < len(cards); j++ {
			if cards[j].GetTitle() < cards[i].GetTitle() {
				cards[i], cards[j] = cards[j], cards[i]
			}
		}
	}
}

// artworkServer serves an image of each ID: a gradient whose colors follow
// from the ID.
func artworkServer(t testing.TB) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/images/")
		h := 0
		for _, c := range id {
			h = (h*31 + int(c)) % len(artworkColors)
		}
		width, height := 200, 300
		if strings.HasSuffix(id, "-thumb") {
			width, height = 320, 180
		}
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		black := color.RGBA{A: 255}
		from, to := artworkColors[h], mix(artworkColors[(h+1)%len(artworkColors)], black, 0.45)
		for y := range height {
			for x := range width {
				t := (float64(x)/float64(width) + float64(y)/float64(height)) / 2
				img.Set(x, y, mix(from, to, t))
			}
		}
		w.Header().Set("Content-Type", "image/png")
		if err := png.Encode(w, img); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// artworkColors are the colors the artwork's gradients start from.
var artworkColors = []color.RGBA{
	{0xC2, 0x4E, 0x3B, 255},
	{0xD0, 0x8A, 0x2E, 255},
	{0x8F, 0xA8, 0x3A, 255},
	{0x3A, 0xA0, 0x6E, 255},
	{0x2E, 0x8C, 0xB0, 255},
	{0x46, 0x5C, 0xC2, 255},
	{0x82, 0x4C, 0xBE, 255},
	{0xB8, 0x48, 0x8E, 255},
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	m := func(x, y uint8) uint8 { return uint8(math.Round(float64(x)*(1-t) + float64(y)*t)) }
	return color.RGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 255}
}

// harness runs an App of the fake core in a tester.
type harness struct {
	t       testing.TB
	app     *App
	core    *fakeCore
	tt      *ui.Tester
	updates chan func()
	// video is the player's latest video.
	video *fakeVideo
	// opened are the URLs opened in the browser.
	opened []string
}

func newHarness(t testing.TB, signedIn bool, width, height int) *harness {
	s := readScreens(t)
	srv := artworkServer(t)
	h := &harness{t: t, core: newFakeCore(s, srv.URL, signedIn), updates: make(chan func(), 256)}
	update := func(fn func()) { h.updates <- fn }
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h.app = New(ctx, Options{
		Core:   h.core,
		Images: images.New(ctx, srv.Client(), t.TempDir(), 200, update),
		Update: update,
		OpenURL: func(url string) error {
			h.opened = append(h.opened, url)
			return nil
		},
		NewVideo: func(changed func(mpv.State)) (Video, error) {
			h.video = &fakeVideo{changed: changed}
			return h.video, nil
		},
	})
	h.tt = ui.NewTester(h.app.View, width, height)
	h.app.opts.SetLocale = h.tt.SetLocale
	h.app.Start()
	h.settle()
	return h
}

// settle runs what the goroutines sent the main thread until they send no
// more for a while, drawing frames between.
func (h *harness) settle() {
	h.t.Helper()
	for {
		select {
		case fn := <-h.updates:
			fn()
			h.tt.Frame()
		case <-time.After(150 * time.Millisecond):
			h.tt.Frame()
			select {
			case fn := <-h.updates:
				fn()
				h.tt.Frame()
				continue
			default:
			}
			// Let the animations end.
			h.tt.Advance(time.Second)
			return
		}
	}
}

// save writes the last frame to dir/name.png when dir is set.
func (h *harness) save(dir, name string) {
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		h.t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, h.tt.Image()); err != nil {
		h.t.Fatal(err)
	}
}

// click scrolls the page until the element showing text or labeled label
// is in view, then clicks it.
func (h *harness) click(label string) {
	h.t.Helper()
	_, height := h.tt.Image().Bounds().Dx(), h.tt.Image().Bounds().Dy()
	for range 30 {
		r, ok := h.tt.Find(label)
		if !ok {
			h.t.Fatalf("click %q: not shown; texts = %q", label, h.tt.Texts())
		}
		mid := r.Y + r.H/2
		if mid > 60 && mid < float32(height)-40 {
			break
		}
		h.tt.Scroll(float32(h.tt.Image().Bounds().Dx())/2+100, float32(height)/2, 0, mid-float32(height)/2)
		h.tt.Frame()
		h.tt.Advance(time.Second)
	}
	if err := h.tt.Click(label); err != nil {
		h.t.Fatalf("click %q: %v", label, err)
	}
	h.settle()
}
