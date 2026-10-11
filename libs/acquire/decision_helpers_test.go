package acquire

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// tid returns the fixed ID numbered n, zero for 0, so that tests can
// name movies, items, profiles and formats as Radarr's fixtures number
// them.
func tid(n int) core.ID {
	var id core.ID
	id[14], id[15] = byte(n>>8), byte(n)
	return id
}

// ixID returns the ID of indexer n, empty for 0.
func ixID(n int) string {
	if n == 0 {
		return ""
	}
	return "indexers/" + strconv.Itoa(n)
}

// fakePorts is a Ports kept in memory. A nil field means none.
type fakePorts struct {
	history     []HistoryEvent // newest first
	queue       []QueueEntry
	pending     map[core.ID][]Release
	blocklisted func(targetID core.ID, r *Release) bool
	indexers    map[string]*IndexerInfo
	free        *int64
	err         error
}

func (f *fakePorts) History(_ context.Context, ids []core.ID) ([]HistoryEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []HistoryEvent
	for _, h := range f.history {
		if slices.Contains(ids, h.ItemID) {
			out = append(out, h)
		}
	}
	return out, nil
}

func (f *fakePorts) Queue(context.Context) ([]QueueEntry, error) { return f.queue, f.err }

func (f *fakePorts) Pending(_ context.Context, id core.ID) ([]Release, error) {
	return f.pending[id], f.err
}

func (f *fakePorts) Blocklisted(_ context.Context, id core.ID, r *Release) (bool, error) {
	if f.blocklisted == nil {
		return false, f.err
	}
	return f.blocklisted(id, r), f.err
}

func (f *fakePorts) Indexer(_ context.Context, id string) (*IndexerInfo, error) {
	return f.indexers[id], f.err
}

func (f *fakePorts) IndexerByName(_ context.Context, name string) (*IndexerInfo, error) {
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

// testNow is the clock of decision tests.
var testNow = time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

// newTestEnv returns an Env at testNow with Radarr's default settings
// and quality definitions, the default delay profile and fake ports.
func newTestEnv() (*Env, *fakePorts) {
	ports := &fakePorts{}
	return &Env{
		Now: func() time.Time { return testNow },
		Config: Config{
			Propers:                   PropersPreferAndUpgrade,
			PreferIndexerFlags:        false,
			CompletedDownloadHandling: true,
			MinimumFreeSpace:          100 << 20,
		},
		Definitions: DefaultDefinitions(release.Movie),
		DelayProfiles: []DelayProfile{{
			ID: tid(1), Default: true, EnableUsenet: true, EnableTorrent: true,
			PreferredProtocol: ProtocolUsenet, Order: 1<<31 - 1,
		}},
		Ports: ports,
	}, ports
}

// testProfile returns a profile of the given qualities, lowest first,
// all allowed, with the cutoff given and upgrades allowed.
func testProfile(cutoff release.Quality, qualities ...release.Quality) *QualityProfile {
	p := &QualityProfile{ID: tid(1), Name: "Test", UpgradeAllowed: true, Cutoff: QualityID(cutoff), Language: release.Any}
	for _, q := range qualities {
		p.Items = append(p.Items, ProfileItem{Quality: q, Allowed: true})
	}
	return p
}

// testMovie returns a candidate for a monitored movie of ID 1 with a
// usenet release of 1 GB published a day before testNow.
func testMovie(p *QualityProfile, q release.Quality) *Candidate {
	return &Candidate{
		Release: &Release{
			GUID: "guid", Title: "Movie.Title.2024.720p.HDTV.x264-Group", Size: 1 << 30,
			PublishDate: testNow.Add(-24 * time.Hour), Protocol: ProtocolUsenet,
			IndexerID: ixID(1), Indexer: "Indexer",
		},
		Quality: NewQualityModel(q),
		Target: &Target{
			ID: tid(1), Kind: release.Movie, Title: "Movie Title", Year: 2024,
			OriginalLanguage: release.English, Profile: p, Monitored: true, Path: "/movies/Movie Title (2024)",
		},
		Items: []*Item{{ID: tid(1), Monitored: true, Runtime: 100}},
	}
}

// check runs a spec and fails the test on an error.
func check(t *testing.T, spec Spec, e *Env, c *Candidate, s *Search) *Rejection {
	t.Helper()
	r, err := spec.Check(context.Background(), e, c, s)
	if err != nil {
		t.Fatalf("%s.Check() error = %v", spec.Name(), err)
	}
	return r
}

// accepted reports whether a spec accepts the candidate.
func accepted(t *testing.T, spec Spec, e *Env, c *Candidate, s *Search) bool {
	t.Helper()
	return check(t, spec, e, c, s) == nil
}

func intPtr(n int) *int { return &n }
