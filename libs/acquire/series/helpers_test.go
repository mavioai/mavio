package series

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// tid returns the fixed ID numbered n, zero for 0, so that tests can
// name series, episodes, files and profiles as Sonarr's fixtures number
// them.
func tid(n int) core.ID {
	var id core.ID
	id[14], id[15] = byte(n>>8), byte(n)
	return id
}

// ids returns the IDs numbered ns.
func ids(ns ...int) []core.ID {
	out := make([]core.ID, len(ns))
	for i, n := range ns {
		out[i] = tid(n)
	}
	return out
}

// ixID returns the ID of indexer n, empty for 0.
func ixID(n int) string {
	if n == 0 {
		return ""
	}
	return "indexers/" + strconv.Itoa(n)
}

// fakePorts is the root Ports kept in memory. A nil field means none.
type fakePorts struct {
	history      []acquire.HistoryEvent // newest first
	queue        []acquire.QueueEntry
	blocklisted  func(targetID core.ID, r *acquire.Release) bool
	indexers     map[string]*acquire.IndexerInfo
	free         *int64
	err          error
	historyCalls int
}

func (f *fakePorts) History(_ context.Context, ids []core.ID) ([]acquire.HistoryEvent, error) {
	f.historyCalls++
	if f.err != nil {
		return nil, f.err
	}
	var out []acquire.HistoryEvent
	for _, h := range f.history {
		if slices.Contains(ids, h.ItemID) {
			out = append(out, h)
		}
	}
	return out, nil
}

func (f *fakePorts) Queue(context.Context) ([]acquire.QueueEntry, error) { return f.queue, f.err }

func (f *fakePorts) Pending(context.Context, core.ID) ([]acquire.Release, error) { return nil, f.err }

func (f *fakePorts) Blocklisted(_ context.Context, id core.ID, r *acquire.Release) (bool, error) {
	if f.blocklisted == nil {
		return false, f.err
	}
	return f.blocklisted(id, r), f.err
}

func (f *fakePorts) Indexer(_ context.Context, id string) (*acquire.IndexerInfo, error) {
	return f.indexers[id], f.err
}

func (f *fakePorts) IndexerByName(_ context.Context, name string) (*acquire.IndexerInfo, error) {
	for _, id := range slices.Sorted(maps.Keys(f.indexers)) {
		if f.indexers[id].Name == name {
			return f.indexers[id], f.err
		}
	}
	return nil, f.err
}

func (f *fakePorts) FreeSpace(context.Context, string) (int64, bool, error) {
	if f.free == nil {
		return 0, false, f.err
	}
	return *f.free, true, f.err
}

// fakeEpisodes is the EpisodePorts kept in memory.
type fakeEpisodes struct {
	bySeason map[int][]*Episode
	inFile   map[core.ID][]core.ID
	// missing are the IDs of files not on disk.
	missing []core.ID
	pending []acquire.Release

	bySeasonCalls, inFileCalls, existsCalls int
}

func (f *fakeEpisodes) EpisodesBySeason(_ context.Context, _ core.ID, season int) ([]*Episode, error) {
	f.bySeasonCalls++
	return f.bySeason[season], nil
}

func (f *fakeEpisodes) EpisodeIDsInFile(_ context.Context, fileID core.ID) ([]core.ID, error) {
	f.inFileCalls++
	return f.inFile[fileID], nil
}

func (f *fakeEpisodes) FileExists(_ context.Context, _ *Series, file *acquire.File) (bool, error) {
	f.existsCalls++
	return !slices.Contains(f.missing, file.ID), nil
}

func (f *fakeEpisodes) Pending(context.Context, core.ID, []core.ID) ([]acquire.Release, error) {
	return f.pending, nil
}

// testNow is the clock of decision tests.
var testNow = time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

// newTestEnv returns an Env at testNow with Sonarr's default settings
// and quality definitions, the default delay profile and fake ports.
func newTestEnv() (*Env, *fakePorts, *fakeEpisodes) {
	ports := &fakePorts{}
	eps := &fakeEpisodes{}
	return &Env{
		Env: &acquire.Env{
			Now: func() time.Time { return testNow },
			Config: acquire.Config{
				Propers:                   acquire.PropersPreferAndUpgrade,
				CompletedDownloadHandling: true,
				MinimumFreeSpace:          100 << 20,
			},
			Definitions: acquire.DefaultDefinitions(release.Series),
			DelayProfiles: []acquire.DelayProfile{{
				ID: tid(1), Default: true, EnableUsenet: true, EnableTorrent: true,
				PreferredProtocol: acquire.ProtocolUsenet, Order: 1<<31 - 1,
			}},
			Ports: ports,
		},
		Episodes: eps,
	}, ports, eps
}

// sonarrQualities are the qualities of Sonarr's
// QualityFixture.GetDefaultQualities, in its order.
var sonarrQualities = []release.Quality{
	release.Unknown, release.SDTV, release.WEBDL480p, release.DVD, release.HDTV720p, release.HDTV1080p,
	release.HDTV2160p, release.RawHD, release.WEBDL720p, release.WEBDL1080p, release.WEBDL2160p,
	release.Bluray720p, release.Bluray1080p, release.Bluray2160p,
}

// defaultQualities returns the items of Sonarr's
// QualityFixture.GetDefaultQualities: the qualities not allowed first,
// then those allowed, all when none are given.
func defaultQualities(allowed ...release.Quality) []acquire.ProfileItem {
	if len(allowed) == 0 {
		allowed = sonarrQualities
	}
	var items []acquire.ProfileItem
	for _, q := range sonarrQualities {
		if !slices.Contains(allowed, q) {
			items = append(items, acquire.ProfileItem{Quality: q})
		}
	}
	for _, q := range allowed {
		items = append(items, acquire.ProfileItem{Quality: q, Allowed: true})
	}
	return items
}

// testProfile returns a profile of the given qualities, lowest first,
// all allowed, with the cutoff given and upgrades allowed.
func testProfile(cutoff release.Quality, qualities ...release.Quality) *acquire.QualityProfile {
	p := &acquire.QualityProfile{ID: tid(1), Name: "Test", UpgradeAllowed: true, Cutoff: acquire.QualityID(cutoff)}
	for _, q := range qualities {
		p.Items = append(p.Items, acquire.ProfileItem{Quality: q, Allowed: true})
	}
	return p
}

// testSeries returns a monitored standard series of ID 1 with a profile.
func testSeries(p *acquire.QualityProfile) *Series {
	return &Series{
		Target: acquire.Target{
			ID: tid(1), Kind: release.Series, Title: "Series Title", Year: 2024,
			OriginalLanguage: release.English, Profile: p, Monitored: true, Path: "/tv/Series Title",
		},
		Runtime: 30,
		Seasons: []int{1},
	}
}

// ep returns a monitored episode of ID id, numbered season×number.
func ep(id, season, number int) *Episode {
	return &Episode{
		Item:          acquire.Item{ID: tid(id), Monitored: true},
		SeasonNumber:  season,
		EpisodeNumber: number,
	}
}

// file returns a file of ID id and quality q.
func file(id int, q acquire.QualityModel) *acquire.File {
	return &acquire.File{ID: tid(id), Quality: q}
}

// model returns a quality at a revision version.
func model(q release.Quality, version int) acquire.QualityModel {
	return acquire.QualityModel{Quality: q, Revision: release.Revision{Version: version}}
}

// testCandidate returns a candidate of a usenet release of 1 GB
// published a day before testNow for episodes of the series.
func testCandidate(s *Series, q acquire.QualityModel, episodes ...*Episode) *Candidate {
	r := &acquire.Release{
		GUID: "guid", Title: "Series.Title.S01E01.720p.HDTV.x264-Group", Size: 1 << 30,
		PublishDate: testNow.Add(-24 * time.Hour), Protocol: acquire.ProtocolUsenet,
		IndexerID: ixID(1), Indexer: "Indexer",
	}
	parsed := &release.ParsedEpisodeInfo{
		SeriesTitle: s.Title, SeasonNumber: 1, Quality: release.ParsedQuality{Quality: q.Quality, Revision: q.Revision},
	}
	for _, e := range episodes {
		parsed.EpisodeNumbers = append(parsed.EpisodeNumbers, e.EpisodeNumber)
	}
	return NewCandidate(r, parsed, s, episodes)
}

// check runs a spec and fails the test on an error.
func check(t *testing.T, spec Spec, e *Env, c *Candidate, s *Search) *acquire.Rejection {
	t.Helper()
	r, err := spec.Check(context.Background(), e, c, s)
	if err != nil {
		t.Fatalf("%s.Check() error = %v", spec.Name(), err)
	}
	return r
}

// want checks that a spec accepts the candidate, or not.
func want(t *testing.T, spec Spec, e *Env, c *Candidate, s *Search, accepted bool) {
	t.Helper()
	r := check(t, spec, e, c, s)
	if got := r == nil; got != accepted {
		t.Errorf("%s accepted = %v (%+v), want = %v", spec.Name(), got, r, accepted)
	}
}

func intPtr(n int) *int { return &n }

// sampleFormatItems returns the scores of Sonarr's
// CustomFormatsTestHelpers.GetSampleFormatItems: the allowed formats
// score 2^i in order, the others -2^(number allowed).
func sampleFormatItems(formats []acquire.CustomFormat, allowed ...string) []acquire.FormatScore {
	var allowedItems, others []acquire.FormatScore
	for _, f := range formats {
		if slices.Contains(allowed, f.Name) {
			allowedItems = append(allowedItems, acquire.FormatScore{FormatID: f.ID, Score: 1 << len(allowedItems)})
		}
	}
	for _, f := range formats {
		if !slices.Contains(allowed, f.Name) {
			others = append(others, acquire.FormatScore{FormatID: f.ID, Score: -(1 << len(allowedItems))})
		}
	}
	return append(others, allowedItems...)
}
