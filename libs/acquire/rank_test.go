package acquire

import (
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/release"
)

// rkFormat1 and rkFormat2 are the custom formats of Radarr's
// PrioritizeDownloadDecisionFixture; its profile scores them 1 and 2.
var (
	rkFormat1 = CustomFormat{ID: tid(1), Name: "My Format 1", Specifications: []Specification{{Kind: SpecLanguage, Value: int(release.English)}}}
	rkFormat2 = CustomFormat{ID: tid(2), Name: "My Format 2", Specifications: []Specification{{Kind: SpecLanguage, Value: int(release.French)}}}
)

// rkDefaultQualities are the qualities of Radarr's
// QualityFixture.GetDefaultQualities, lowest first.
var rkDefaultQualities = []release.Quality{
	release.Unknown, release.Cam, release.Telecine, release.DVDSCR, release.Regional,
	release.SDTV, release.DVD, release.DVDR, release.HDTV720p, release.HDTV1080p,
	release.HDTV2160p, release.WEBDL480p, release.WEBDL720p, release.WEBDL1080p,
	release.WEBDL2160p, release.Bluray480p, release.Bluray576p, release.Bluray720p,
	release.Bluray1080p, release.Bluray2160p, release.BRDisk, release.RawHD,
}

// rkRankEnv returns the Env of the prioritization tests: usenet
// preferred, no preferred sizes.
func rkRankEnv(preferred Protocol) *Env {
	e, _ := newTestEnv()
	e.DelayProfiles = []DelayProfile{{ID: tid(1), Default: true, EnableUsenet: true, EnableTorrent: true, PreferredProtocol: preferred, Order: 1<<31 - 1}}
	e.Definitions = nil
	return e
}

// rkPreferredSize gives every quality a preferred size in megabytes per
// minute.
func rkPreferredSize(e *Env, size float64) {
	e.Definitions = nil
	for _, q := range rkDefaultQualities {
		e.Definitions = append(e.Definitions, Definition{Quality: q, PreferredSize: size})
	}
}

type rkRemote struct {
	age             int // days
	size            int64
	protocol        Protocol
	runtime         int
	indexerPriority int
}

// rkDecision returns a decision on a release of quality, as Radarr's
// GivenRemoteMovie with its defaults: usenet, a runtime of 150 minutes
// and an indexer priority of 25.
func rkDecision(q QualityModel, opts ...func(*rkRemote)) Decision {
	o := rkRemote{protocol: ProtocolUsenet, runtime: 150, indexerPriority: 25}
	for _, f := range opts {
		f(&o)
	}
	p := testProfile(release.Unknown, rkDefaultQualities...)
	p.FormatItems = []FormatScore{{FormatID: rkFormat1.ID, Score: 1}, {FormatID: rkFormat2.ID, Score: 2}}
	return Decision{Candidate: &Candidate{
		Release: &Release{
			Title: "A Movie 1998", PublishDate: testNow.AddDate(0, 0, -o.age), Size: o.size,
			Protocol: o.protocol, IndexerPriority: o.indexerPriority,
		},
		Quality: q,
		Year:    1998,
		Target:  &Target{ID: tid(1), Kind: release.Movie, Title: "A Movie", Profile: p, Monitored: true},
		Items:   []*Item{{ID: tid(1), Monitored: true, Runtime: o.runtime}},
	}}
}

func rkAge(days int) func(*rkRemote)          { return func(o *rkRemote) { o.age = days } }
func rkSize(mb int64) func(*rkRemote)         { return func(o *rkRemote) { o.size = mb << 20 } }
func rkProtocol(p Protocol) func(*rkRemote)   { return func(o *rkRemote) { o.protocol = p } }
func rkRuntime(minutes int) func(*rkRemote)   { return func(o *rkRemote) { o.runtime = minutes } }
func rkIndexerPriority(n int) func(*rkRemote) { return func(o *rkRemote) { o.indexerPriority = n } }

// rkTorrent turns a decision's release into a torrent published at
// testNow, as the TorrentInfo of the seeders and peers cases.
func rkTorrent(d Decision, seeders, peers *int, size int64) Decision {
	d.Candidate.Release.Protocol = ProtocolTorrent
	d.Candidate.Release.PublishDate = testNow
	d.Candidate.Release.Size = size
	d.Candidate.Release.Seeders = seeders
	d.Candidate.Release.Peers = peers
	return d
}

// rkWithFormats gives a decision's candidate formats and their score.
func rkWithFormats(d Decision, formats ...CustomFormat) Decision {
	d.Candidate.Formats = formats
	d.Candidate.FormatScore = d.Candidate.Target.Profile.FormatScore(formats)
	return d
}

func rkRevision(q release.Quality, version, real int) QualityModel {
	return QualityModel{Quality: q, Revision: release.Revision{Version: version, Real: real}}
}

// rkIndex returns where a decision's candidate is in decisions, or -1.
func rkIndex(decisions []Decision, d Decision) int {
	for i := range decisions {
		if decisions[i].Candidate == d.Candidate {
			return i
		}
	}
	return -1
}

// TestPrioritize ports Radarr's PrioritizeDownloadDecisionFixture: each
// case lists decisions in the order given and wants the first of the
// result, or the whole order.
func TestPrioritize(t *testing.T) {
	tests := []struct {
		name  string
		setup func(e *Env)
		// decisions returns the decisions and the order wanted, by index
		// into them; a single index wants only the first.
		decisions func() ([]Decision, []int)
	}{
		{
			name: "should_put_reals_before_non_reals",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkDecision(rkRevision(release.HDTV720p, 1, 0)),
					rkDecision(rkRevision(release.HDTV720p, 1, 1)),
				}, []int{1}
			},
		},
		{
			name: "should_put_propers_before_non_propers",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkDecision(rkRevision(release.HDTV720p, 1, 0)),
					rkDecision(rkRevision(release.HDTV720p, 2, 0)),
				}, []int{1}
			},
		},
		{
			name: "should_put_higher_quality_before_lower",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkDecision(NewQualityModel(release.SDTV)),
					rkDecision(NewQualityModel(release.HDTV720p)),
				}, []int{1}
			},
		},
		{
			name: "should_order_by_age_then_largest_rounded_to_200mb",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkDecision(NewQualityModel(release.SDTV), rkSize(100), rkAge(1)),
					rkDecision(NewQualityModel(release.HDTV720p), rkSize(1200), rkAge(1000)),
					rkDecision(NewQualityModel(release.HDTV720p), rkSize(1250), rkAge(10)),
					rkDecision(NewQualityModel(release.HDTV720p), rkSize(3000), rkAge(1)),
				}, []int{3}
			},
		},
		{
			// 2 MB/min * 150 min = 300 MB.
			name:  "should_order_by_closest_to_preferred_size_if_both_over",
			setup: func(e *Env) { rkPreferredSize(e, 2) },
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkDecision(NewQualityModel(release.HDTV720p), rkSize(400), rkAge(1)),
					rkDecision(NewQualityModel(release.HDTV720p), rkSize(15000), rkAge(1)),
				}, []int{0}
			},
		},
		{
			name:  "should_order_by_largest_to_if_zero_runtime",
			setup: func(e *Env) { rkPreferredSize(e, 2) },
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkDecision(NewQualityModel(release.HDTV720p), rkSize(400), rkAge(1), rkRuntime(0)),
					rkDecision(NewQualityModel(release.HDTV720p), rkSize(15000), rkAge(1), rkRuntime(0)),
				}, []int{1}
			},
		},
		{
			// 390 MB/min * 150 min = 58,500 MB.
			name:  "should_order_by_closest_to_preferred_size_if_both_under",
			setup: func(e *Env) { rkPreferredSize(e, 390) },
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkDecision(NewQualityModel(release.HDTV720p), rkSize(100), rkAge(1)),
					rkDecision(NewQualityModel(release.HDTV720p), rkSize(15000), rkAge(1)),
				}, []int{1}
			},
		},
		{
			// 46 MB/min * 150 min = 6900 MB.
			name:  "should_order_by_closest_to_preferred_size_if_preferred_is_in_between",
			setup: func(e *Env) { rkPreferredSize(e, 46) },
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkDecision(NewQualityModel(release.HDTV720p), rkSize(100), rkAge(1)),
					rkDecision(NewQualityModel(release.HDTV720p), rkSize(500), rkAge(1)),
					rkDecision(NewQualityModel(release.HDTV720p), rkSize(7000), rkAge(1)),
					rkDecision(NewQualityModel(release.HDTV720p), rkSize(15000), rkAge(1)),
				}, []int{2}
			},
		},
		{
			name: "should_order_by_youngest",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkDecision(NewQualityModel(release.HDTV720p), rkAge(10)),
					rkDecision(NewQualityModel(release.HDTV720p), rkAge(5)),
				}, []int{1}
			},
		},
		{
			name: "should_put_usenet_above_torrent_when_usenet_is_preferred",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkDecision(NewQualityModel(release.HDTV720p), rkProtocol(ProtocolTorrent)),
					rkDecision(NewQualityModel(release.HDTV720p), rkProtocol(ProtocolUsenet)),
				}, []int{1}
			},
		},
		{
			name:  "should_put_torrent_above_usenet_when_torrent_is_preferred",
			setup: func(e *Env) { e.DelayProfiles[0].PreferredProtocol = ProtocolTorrent },
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkDecision(NewQualityModel(release.HDTV720p), rkProtocol(ProtocolTorrent)),
					rkDecision(NewQualityModel(release.HDTV720p), rkProtocol(ProtocolUsenet)),
				}, []int{0}
			},
		},
		{
			name: "should_prefer_releases_with_more_seeders",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkTorrent(rkDecision(NewQualityModel(release.HDTV720p)), intPtr(10), nil, 0),
					rkTorrent(rkDecision(NewQualityModel(release.HDTV720p)), intPtr(100), nil, 0),
				}, []int{1}
			},
		},
		{
			name: "should_prefer_releases_with_more_peers_given_equal_number_of_seeds",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkTorrent(rkDecision(NewQualityModel(release.HDTV720p)), intPtr(10), intPtr(10), 0),
					rkTorrent(rkDecision(NewQualityModel(release.HDTV720p)), intPtr(10), intPtr(100), 0),
				}, []int{1}
			},
		},
		{
			name: "should_prefer_releases_with_more_peers_no_seeds",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkTorrent(rkDecision(NewQualityModel(release.HDTV720p)), intPtr(0), intPtr(10), 0),
					rkTorrent(rkDecision(NewQualityModel(release.HDTV720p)), intPtr(0), intPtr(100), 0),
				}, []int{1}
			},
		},
		{
			// Radarr's fixture sets the first release's size to 250 MB
			// after cloning it, so the second keeps 200 MB.
			name: "should_prefer_first_release_if_peers_and_size_are_too_similar",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkTorrent(rkDecision(NewQualityModel(release.HDTV720p)), intPtr(1000), intPtr(10), 250<<20),
					rkTorrent(rkDecision(NewQualityModel(release.HDTV720p)), intPtr(1100), intPtr(10), 200<<20),
				}, []int{0}
			},
		},
		{
			name: "should_prefer_first_release_if_age_and_size_are_too_similar",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkDecision(NewQualityModel(release.HDTV720p), rkAge(100), rkSize(200)),
					rkDecision(NewQualityModel(release.HDTV720p), rkAge(150), rkSize(250)),
				}, []int{0}
			},
		},
		{
			name: "should_prefer_better_custom_format",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkDecision(NewQualityModel(release.Bluray720p)),
					rkWithFormats(rkDecision(NewQualityModel(release.Bluray720p)), rkFormat1),
				}, []int{1}
			},
		},
		{
			name: "should_prefer_better_custom_format2",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkWithFormats(rkDecision(NewQualityModel(release.Bluray720p)), rkFormat1),
					rkWithFormats(rkDecision(NewQualityModel(release.Bluray720p)), rkFormat2),
				}, []int{1}
			},
		},
		{
			name: "should_prefer_2_custom_formats",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkWithFormats(rkDecision(NewQualityModel(release.Bluray720p)), rkFormat1),
					rkWithFormats(rkDecision(NewQualityModel(release.Bluray720p)), rkFormat1, rkFormat2),
				}, []int{1}
			},
		},
		{
			name:  "should_prefer_proper_over_score_when_download_propers_is_prefer_and_upgrade",
			setup: func(e *Env) { e.Config.Propers = PropersPreferAndUpgrade },
			decisions: func() ([]Decision, []int) {
				return rkScoredRevisions(rkRevision(release.WEBDL1080p, 1, 0), rkRevision(release.WEBDL1080p, 2, 0)), []int{1}
			},
		},
		{
			name:  "should_prefer_proper_over_score_when_download_propers_is_do_not_upgrade",
			setup: func(e *Env) { e.Config.Propers = PropersDoNotUpgrade },
			decisions: func() ([]Decision, []int) {
				return rkScoredRevisions(rkRevision(release.WEBDL1080p, 1, 0), rkRevision(release.WEBDL1080p, 2, 0)), []int{1}
			},
		},
		{
			name:  "should_prefer_score_over_proper_when_download_propers_is_do_not_prefer",
			setup: func(e *Env) { e.Config.Propers = PropersDoNotPrefer },
			decisions: func() ([]Decision, []int) {
				return rkScoredRevisions(rkRevision(release.WEBDL1080p, 1, 0), rkRevision(release.WEBDL1080p, 2, 0)), []int{0}
			},
		},
		{
			name:  "should_prefer_score_over_real_when_download_propers_is_do_not_prefer",
			setup: func(e *Env) { e.Config.Propers = PropersDoNotPrefer },
			decisions: func() ([]Decision, []int) {
				return rkScoredRevisions(rkRevision(release.WEBDL1080p, 1, 0), rkRevision(release.WEBDL1080p, 1, 1)), []int{0}
			},
		},
		{
			name: "sort_download_decisions_based_on_indexer_priority",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkDecision(NewQualityModel(release.WEBDL1080p), rkIndexerPriority(25)),
					rkDecision(NewQualityModel(release.WEBDL1080p), rkIndexerPriority(50)),
					rkDecision(NewQualityModel(release.WEBDL1080p), rkIndexerPriority(1)),
				}, []int{2, 0, 1}
			},
		},
		{
			name: "ensure_download_decisions_indexer_priority_is_not_perfered_over_quality",
			decisions: func() ([]Decision, []int) {
				return []Decision{
					rkDecision(NewQualityModel(release.HDTV720p), rkIndexerPriority(25)),
					rkDecision(NewQualityModel(release.WEBDL1080p), rkIndexerPriority(50)),
					rkDecision(NewQualityModel(release.SDTV), rkIndexerPriority(1)),
					rkDecision(NewQualityModel(release.WEBDL1080p), rkIndexerPriority(25)),
				}, []int{3, 1, 0, 2}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := rkRankEnv(ProtocolUsenet)
			if tt.setup != nil {
				tt.setup(e)
			}
			decisions, want := tt.decisions()
			got := e.Prioritize(decisions)
			if len(got) != len(decisions) {
				t.Fatalf("len(Prioritize()) = %d, want = %d", len(got), len(decisions))
			}
			for i, w := range want {
				if g := rkIndex(decisions, got[i]); g != w {
					t.Errorf("Prioritize()[%d] = decision %d, want = decision %d", i, g, w)
				}
			}
		})
	}
}

// rkScoredRevisions returns decisions on two releases, the first scoring
// 10 and the second 0, as the propers cases of Radarr's fixture.
func rkScoredRevisions(a, b QualityModel) []Decision {
	x, y := rkDecision(a), rkDecision(b)
	x.Candidate.FormatScore = 10
	return []Decision{x, y}
}

// TestPrioritizeUnknownTargetsLast checks that Prioritize keeps decisions
// on candidates matching no target after the others, as Radarr's Union of
// the decisions without a movie.
func TestPrioritizeUnknownTargetsLast(t *testing.T) {
	e := rkRankEnv(ProtocolUsenet)
	unknown := Decision{Candidate: &Candidate{Release: &Release{Title: "Unknown", PublishDate: testNow.Add(-time.Hour)}}}
	low := rkDecision(NewQualityModel(release.SDTV))
	high := rkDecision(NewQualityModel(release.HDTV720p))
	decisions := []Decision{unknown, low, high}
	got := e.Prioritize(decisions)
	for i, w := range []int{2, 1, 0} {
		if g := rkIndex(decisions, got[i]); g != w {
			t.Errorf("Prioritize()[%d] = decision %d, want = decision %d", i, g, w)
		}
	}
}
