package acquire

import (
	"testing"

	"github.com/mavioai/mavio/libs/release"
)

// defaultQualities ports Radarr's QualityFixture.GetDefaultQualities: the
// qualities not allowed first, then the allowed ones in their order.
func defaultQualities(allowed ...release.Quality) []ProfileItem {
	qualities := []release.Quality{
		release.Unknown, release.Cam, release.Telecine, release.DVDSCR, release.Regional, release.SDTV, release.DVD,
		release.DVDR, release.HDTV720p, release.HDTV1080p, release.HDTV2160p, release.WEBDL480p, release.WEBDL720p,
		release.WEBDL1080p, release.WEBDL2160p, release.Bluray480p, release.Bluray576p, release.Bluray720p,
		release.Bluray1080p, release.Bluray2160p, release.BRDisk, release.RawHD,
	}
	if len(allowed) == 0 {
		allowed = qualities
	}
	var items []ProfileItem
	for _, q := range qualities {
		if !contains(allowed, q) {
			items = append(items, ProfileItem{Quality: q})
		}
	}
	for _, q := range allowed {
		items = append(items, ProfileItem{Quality: q, Allowed: true})
	}
	return items
}

func contains(qs []release.Quality, q release.Quality) bool {
	for _, x := range qs {
		if x == q {
			return true
		}
	}
	return false
}

func groupedProfile() *QualityProfile {
	return &QualityProfile{Items: []ProfileItem{
		{Quality: release.SDTV},
		{Quality: release.DVD},
		{Allowed: true, Items: []ProfileItem{
			{Quality: release.HDTV720p, Allowed: true},
			{Quality: release.WEBDL720p, Allowed: true},
		}},
		{Quality: release.Bluray720p, Allowed: true},
	}}
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// TestQualityIndexCompare ports Radarr's QualityIndexCompareToFixture.
func TestQualityIndexCompare(t *testing.T) {
	tests := []struct {
		left, right   QualityIndex
		respect, want int
	}{
		{QualityIndex{1, 0}, QualityIndex{1, 0}, 1, 0},
		{QualityIndex{1, 1}, QualityIndex{1, 0}, 1, 1},
		{QualityIndex{2, 0}, QualityIndex{1, 0}, 1, 1},
		{QualityIndex{1, 0}, QualityIndex{1, 1}, 1, -1},
		{QualityIndex{1, 0}, QualityIndex{2, 0}, 1, -1},
		{QualityIndex{1, 0}, QualityIndex{1, 0}, 0, 0},
		{QualityIndex{1, 1}, QualityIndex{1, 0}, 0, 0},
		{QualityIndex{2, 0}, QualityIndex{1, 0}, 0, 1},
		{QualityIndex{1, 0}, QualityIndex{1, 1}, 0, 0},
		{QualityIndex{1, 0}, QualityIndex{2, 0}, 0, -1},
	}
	for _, tt := range tests {
		if got := tt.left.Compare(tt.right, tt.respect == 1); got != tt.want {
			t.Errorf("%v.Compare(%v, %v) = %d, want = %d", tt.left, tt.right, tt.respect == 1, got, tt.want)
		}
	}
}

// TestCompare ports Radarr's QualityModelComparerFixture.
func TestCompare(t *testing.T) {
	def := &QualityProfile{Items: defaultQualities()}
	custom := &QualityProfile{Items: defaultQualities(release.Bluray720p, release.DVD)}
	grouped := groupedProfile()
	v2 := QualityModel{Quality: release.Bluray1080p, Revision: release.Revision{Version: 2}}
	tests := []struct {
		name    string
		profile *QualityProfile
		a, b    QualityModel
		respect bool
		want    int
	}{
		{"greater quality", def, NewQualityModel(release.Bluray1080p), NewQualityModel(release.DVD), false, 1},
		{"lesser quality", def, NewQualityModel(release.DVD), NewQualityModel(release.Bluray1080p), false, -1},
		{"proper of the same quality", def, v2, NewQualityModel(release.Bluray1080p), false, 1},
		{"custom profile", custom, NewQualityModel(release.DVD), NewQualityModel(release.Bluray720p), false, 1},
		{"group order ignored", grouped, NewQualityModel(release.HDTV720p), NewQualityModel(release.WEBDL720p), false, 0},
		{"group order respected", grouped, NewQualityModel(release.HDTV720p), NewQualityModel(release.WEBDL720p), true, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sign(tt.profile.Compare(tt.a, tt.b, tt.respect)); got != tt.want {
				t.Errorf("Compare(%v, %v) = %d, want = %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestRevisionCompare ports Radarr's RevisionComparableFixture.
func TestRevisionCompare(t *testing.T) {
	r := func(version, real int) release.Revision { return release.Revision{Version: version, Real: real} }
	tests := []struct {
		name string
		a, b release.Revision
		want int
	}{
		{"real", r(1, 1), r(1, 0), 1},
		{"proper", r(2, 0), r(1, 0), 1},
		{"proper of a real", r(2, 1), r(1, 1), 1},
		{"second real", r(1, 0), r(1, 1), -1},
		{"second proper", r(1, 0), r(2, 0), -1},
		{"second proper of a real", r(1, 1), r(2, 1), -1},
		{"equal", r(1, 0), r(1, 0), 0},
		{"equal real", r(1, 1), r(1, 1), 0},
		{"equal real proper", r(2, 1), r(2, 1), 0},
	}
	for _, tt := range tests {
		if got := sign(tt.a.Compare(tt.b)); got != tt.want {
			t.Errorf("%s: Compare(%+v, %+v) = %d, want = %d", tt.name, tt.a, tt.b, got, tt.want)
		}
	}
}

func TestAllowedQualities(t *testing.T) {
	p := groupedProfile()
	if got := p.FirstAllowedQuality(); got != release.HDTV720p {
		t.Errorf("FirstAllowedQuality() = %v, want = %v", got, release.HDTV720p)
	}
	if got := p.LastAllowedQuality(); got != release.Bluray720p {
		t.Errorf("LastAllowedQuality() = %v, want = %v", got, release.Bluray720p)
	}
	for q, want := range map[release.Quality]bool{release.SDTV: false, release.WEBDL720p: true, release.Remux2160p: false} {
		if got := p.Allowed(q); got != want {
			t.Errorf("Allowed(%v) = %v, want = %v", q, got, want)
		}
	}
}

func TestDefaultProfiles(t *testing.T) {
	formats := []CustomFormat{{ID: tid(3), Name: "x"}}
	for _, kind := range []release.Kind{release.Movie, release.Series} {
		profiles := DefaultProfiles(kind, formats)
		if len(profiles) != 6 {
			t.Fatalf("kind %d: len(DefaultProfiles) = %d, want = 6", kind, len(profiles))
		}
		for _, p := range profiles {
			if err := p.Validate(kind); err != nil {
				t.Errorf("kind %d: profile %q: Validate() = %v", kind, p.Name, err)
			}
			if len(p.FormatItems) != 1 || p.FormatItems[0] != (FormatScore{FormatID: tid(3)}) {
				t.Errorf("kind %d: profile %q: FormatItems = %v, want = [{3 0}]", kind, p.Name, p.FormatItems)
			}
			if p.MinUpgradeFormatScore != 1 {
				t.Errorf("kind %d: profile %q: MinUpgradeFormatScore = %d, want = 1", kind, p.Name, p.MinUpgradeFormatScore)
			}
		}
	}

	movie := DefaultProfiles(release.Movie, nil)
	// HD-720p's cutoff Bluray-720p stands alone; Any's Bluray-480p too.
	if got, want := movie[2].Cutoff, QualityID(release.Bluray720p); got != want {
		t.Errorf("HD-720p cutoff = %d, want = %d", got, want)
	}
	if got := movie[0].Language; got != release.Original {
		t.Errorf("movie profile language = %v, want = %v", got, release.Original)
	}
	// Sonarr's HD-720p cuts off at HDTV-720p; its WEB 720p group is the
	// second group, ID 1001.
	series := DefaultProfiles(release.Series, nil)
	if got, want := series[2].Cutoff, QualityID(release.HDTV720p); got != want {
		t.Errorf("series HD-720p cutoff = %d, want = %d", got, want)
	}
	i := series[2].Index(release.WEBDL720p, false).Index
	if it := series[2].Items[i]; it.ID != 1001 || it.Name != "WEB 720p" || !it.Allowed {
		t.Errorf("series HD-720p WEB 720p item = %+v, want = group 1001 allowed", it)
	}
	// A cutoff inside a group is the group.
	p := NewProfile(release.Movie, "web", release.WEBRip1080p, []release.Quality{release.WEBRip1080p}, nil)
	if got := p.Cutoff; got != 1002 {
		t.Errorf("NewProfile cutoff = %d, want = 1002", got)
	}
	if got := p.Index(release.WEBDL1080p, false); got != p.IndexOf(1002, false) {
		t.Errorf("Index(WEBDL-1080p) = %v, want = the group's %v", got, p.IndexOf(1002, false))
	}
}

func TestValidateProfile(t *testing.T) {
	base := func() QualityProfile {
		return NewProfile(release.Series, "p", release.HDTV720p, []release.Quality{release.HDTV720p}, nil)
	}
	tests := []struct {
		name   string
		modify func(p *QualityProfile)
	}{
		{"no name", func(p *QualityProfile) { p.Name = "" }},
		{"nothing allowed", func(p *QualityProfile) {
			for i := range p.Items {
				p.Items[i].Allowed = false
			}
		}},
		{"cutoff not allowed", func(p *QualityProfile) { p.Cutoff = QualityID(release.SDTV) }},
		{"cutoff missing", func(p *QualityProfile) { p.Cutoff = 999 }},
		{"quality missing", func(p *QualityProfile) { p.Items = p.Items[1:] }},
		{"quality repeated", func(p *QualityProfile) { p.Items = append(p.Items, ProfileItem{Quality: release.SDTV}) }},
		{"group of one", func(p *QualityProfile) {
			i := p.Index(release.WEBDL480p, false).Index
			p.Items[i].Items = p.Items[i].Items[:1]
			p.Items = append(p.Items, ProfileItem{Quality: release.WEBDL480p})
		}},
		{"group ID below 1000", func(p *QualityProfile) { p.Items[p.Index(release.WEBDL480p, false).Index].ID = 5 }},
	}
	ok := base()
	if err := ok.Validate(release.Series); err != nil {
		t.Fatalf("Validate() = %v, want = nil", err)
	}
	for _, tt := range tests {
		p := base()
		tt.modify(&p)
		if err := p.Validate(release.Series); err == nil {
			t.Errorf("%s: Validate() = nil, want = an error", tt.name)
		}
	}
}

func TestQualityIDs(t *testing.T) {
	if len(qualityIDs) != len(Qualities) {
		t.Fatalf("len(qualityIDs) = %d, want = %d", len(qualityIDs), len(Qualities))
	}
	for _, q := range Qualities {
		got, ok := QualityByID(QualityID(q))
		if !ok || got != q {
			t.Errorf("QualityByID(QualityID(%v)) = %v, %v, want = %v, true", q, got, ok, q)
		}
	}
	if got := QualityID(release.Remux2160p); got != 31 {
		t.Errorf("QualityID(Remux-2160p) = %d, want = 31", got)
	}
}

func TestFormatScore(t *testing.T) {
	p := QualityProfile{FormatItems: []FormatScore{{tid(1), 10}, {tid(2), -5}, {tid(3), 100}}}
	if got := p.FormatScore([]CustomFormat{{ID: tid(1)}, {ID: tid(2)}}); got != 5 {
		t.Errorf("FormatScore() = %d, want = 5", got)
	}
}
