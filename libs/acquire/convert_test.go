package acquire

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

func TestQualityProfileConversion(t *testing.T) {
	tests := []struct {
		name string
		p    core.QualityProfile
	}{
		{"movie default", mustProfileToCore(t, NewProfile(release.Movie, "HD-1080p", release.Bluray1080p,
			[]release.Quality{release.HDTV1080p, release.WEBDL1080p, release.WEBRip1080p, release.Bluray1080p},
			[]CustomFormat{{ID: tid(1)}, {ID: tid(2)}}), core.KindMovie)},
		{"series default", mustProfileToCore(t, DefaultProfiles(release.Series, nil)[0], core.KindSeries)},
		{"every field", core.QualityProfile{
			ID: tid(7), Kind: core.KindMovie, Name: "Custom", UpgradeAllowed: true, Cutoff: 1001,
			Items: []core.QualityProfileItem{
				{Quality: 1, Allowed: false},
				{ID: 1001, Name: "WEB", Allowed: true, Items: []core.QualityProfileItem{
					{Quality: 3, Allowed: true}, {Quality: 15, Allowed: true},
				}},
				{Quality: 31, Allowed: true},
			},
			MinFormatScore: -10, CutoffFormatScore: 100, MinUpgradeFormatScore: 5,
			FormatScores: []core.FormatScore{{FormatID: tid(1), Score: 10}, {FormatID: tid(2), Score: -5}},
			Language:     "Portuguese (Brazil)",
		}},
		{"any language", core.QualityProfile{
			ID: tid(8), Kind: core.KindSeries, Name: "Any", Cutoff: 1,
			Items: []core.QualityProfileItem{{Quality: 1, Allowed: true}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := QualityProfileFromCore(tt.p)
			if err != nil {
				t.Fatalf("QualityProfileFromCore() error = %v", err)
			}
			got, err := QualityProfileToCore(p, tt.p.Kind)
			if err != nil {
				t.Fatalf("QualityProfileToCore() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.p) {
				t.Errorf("round trip got = %+v, want = %+v", got, tt.p)
			}
		})
	}
}

func mustProfileToCore(t *testing.T, p QualityProfile, kind core.ItemKind) core.QualityProfile {
	t.Helper()
	p.ID = tid(1)
	out, err := QualityProfileToCore(p, kind)
	if err != nil {
		t.Fatalf("QualityProfileToCore() error = %v", err)
	}
	return out
}

func TestQualityProfileFromCoreLanguage(t *testing.T) {
	tests := []struct {
		language string
		want     release.Language
	}{
		{"", release.Any},
		{"Any", release.Any},
		{"Unknown", release.LanguageUnknown},
		{"Original", release.Original},
		{"French", release.French},
	}
	for _, tt := range tests {
		p, err := QualityProfileFromCore(core.QualityProfile{Language: tt.language})
		if err != nil || p.Language != tt.want {
			t.Errorf("QualityProfileFromCore(%q).Language got = %v, %v, want = %v, nil", tt.language, p.Language, err, tt.want)
		}
	}
}

// TestQualityProfileToCoreLanguage checks that any language is stored as
// empty and the unknown language apart from it.
func TestQualityProfileToCoreLanguage(t *testing.T) {
	tests := []struct {
		language release.Language
		want     string
	}{
		{release.Any, ""},
		{release.LanguageUnknown, "Unknown"},
		{release.Original, "Original"},
	}
	for _, tt := range tests {
		p, err := QualityProfileToCore(QualityProfile{Language: tt.language}, core.KindMovie)
		if err != nil || p.Language != tt.want {
			t.Errorf("QualityProfileToCore(%v).Language got = %q, %v, want = %q, nil", tt.language, p.Language, err, tt.want)
		}
	}
}

func TestQualityProfileConversionErrors(t *testing.T) {
	tests := []struct {
		name string
		p    core.QualityProfile
	}{
		{"unknown quality", core.QualityProfile{Items: []core.QualityProfileItem{{Quality: 99}}}},
		{"unknown quality in group", core.QualityProfile{Items: []core.QualityProfileItem{
			{ID: 1000, Name: "G", Items: []core.QualityProfileItem{{Quality: 1}, {Quality: 98}}},
		}}},
		{"unknown language", core.QualityProfile{Language: "Klingon"}},
	}
	for _, tt := range tests {
		if _, err := QualityProfileFromCore(tt.p); !errors.Is(err, ErrUnknownValue) {
			t.Errorf("QualityProfileFromCore(%s) error got = %v, want = %v", tt.name, err, ErrUnknownValue)
		}
	}
	if _, err := QualityProfileToCore(QualityProfile{Language: release.Any + 1}, core.KindMovie); !errors.Is(err, ErrUnknownValue) {
		t.Errorf("QualityProfileToCore(unknown language) error got = %v, want = %v", err, ErrUnknownValue)
	}
}

func TestCustomFormatConversion(t *testing.T) {
	spec := func(kind core.CustomFormatSpecKind, value string) core.CustomFormatSpec {
		return core.CustomFormatSpec{Kind: kind, Name: string(kind), Value: value}
	}
	tests := []struct {
		name string
		spec core.CustomFormatSpec
		want Specification
	}{
		{"release title", spec(core.SpecReleaseTitle, `\bx265\b`), Specification{Kind: SpecReleaseTitle, Pattern: `\bx265\b`}},
		{"edition", spec(core.SpecEdition, `(?i)extended`), Specification{Kind: SpecEdition, Pattern: `(?i)extended`}},
		{"release group", spec(core.SpecReleaseGroup, `^FLUX$`), Specification{Kind: SpecReleaseGroup, Pattern: `^FLUX$`}},
		{"language", spec(core.SpecLanguage, "Spanish (Latino)"), Specification{Kind: SpecLanguage, Value: int(release.SpanishLatino)}},
		{
			"language original except",
			core.CustomFormatSpec{Kind: core.SpecLanguage, Name: "language", Value: "Original", ExceptLanguage: true},
			Specification{Kind: SpecLanguage, Value: int(release.Original), ExceptLanguage: true},
		},
		{"language unknown", spec(core.SpecLanguage, "Unknown"), Specification{Kind: SpecLanguage, Value: int(release.LanguageUnknown)}},
		{"indexer flag", spec(core.SpecIndexerFlag, FlagFreeleech), Specification{Kind: SpecIndexerFlag, Flag: FlagFreeleech}},
		{"source", spec(core.SpecSource, "webrip"), Specification{Kind: SpecSource, Value: int(release.SourceWebRip)}},
		{"source unknown", spec(core.SpecSource, "unknown"), Specification{Kind: SpecSource, Value: int(release.SourceUnknown)}},
		{"resolution", spec(core.SpecResolution, "2160"), Specification{Kind: SpecResolution, Value: 2160}},
		{"quality modifier", spec(core.SpecQualityModifier, "remux"), Specification{Kind: SpecQualityModifier, Value: int(release.ModifierRemux)}},
		{
			"size",
			core.CustomFormatSpec{Kind: core.SpecSize, Name: "size", Min: 1.5, Max: 20},
			Specification{Kind: SpecSize, Min: 1.5, Max: 20},
		},
		{
			"year",
			core.CustomFormatSpec{Kind: core.SpecYear, Name: "year", Min: 1990, Max: 1999},
			Specification{Kind: SpecYear, Min: 1990, Max: 1999},
		},
		{"release type", spec(core.SpecReleaseType, "season_pack"), Specification{Kind: SpecReleaseType, Value: int(release.SeasonPack)}},
		{
			"negated required",
			core.CustomFormatSpec{Kind: core.SpecReleaseTitle, Name: "release_title", Value: "x", Negate: true, Required: true},
			Specification{Kind: SpecReleaseTitle, Pattern: "x", Negate: true, Required: true},
		},
	}
	covered := map[core.CustomFormatSpecKind]bool{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			covered[tt.spec.Kind] = true
			cf := core.CustomFormat{ID: tid(3), Name: "Format", IncludeWhenRenaming: true, Specifications: []core.CustomFormatSpec{tt.spec}}
			f, err := CustomFormatFromCore(cf)
			if err != nil {
				t.Fatalf("CustomFormatFromCore() error = %v", err)
			}
			tt.want.Name = tt.spec.Name
			if f.ID != cf.ID || f.Name != cf.Name || !f.IncludeWhenRenaming || len(f.Specifications) != 1 ||
				!reflect.DeepEqual(f.Specifications[0], tt.want) {
				t.Errorf("CustomFormatFromCore() got = %+v, want = specification %+v", f, tt.want)
			}
			if err := f.Validate(); err != nil {
				t.Errorf("Validate() error = %v", err)
			}
			got, err := CustomFormatToCore(f)
			if err != nil {
				t.Fatalf("CustomFormatToCore() error = %v", err)
			}
			if !reflect.DeepEqual(got, cf) {
				t.Errorf("round trip got = %+v, want = %+v", got, cf)
			}
		})
	}
	for _, k := range core.CustomFormatSpecKinds {
		if !covered[k] {
			t.Errorf("kind %q has no conversion case", k)
		}
	}
}

func TestCustomFormatConversionErrors(t *testing.T) {
	tests := []struct {
		name string
		spec core.CustomFormatSpec
	}{
		{"kind", core.CustomFormatSpec{Kind: "codec", Value: "x265"}},
		{"language", core.CustomFormatSpec{Kind: core.SpecLanguage, Value: "Klingon"}},
		{"source", core.CustomFormatSpec{Kind: core.SpecSource, Value: "vhs"}},
		{"resolution", core.CustomFormatSpec{Kind: core.SpecResolution, Value: "4k"}},
		{"quality modifier", core.CustomFormatSpec{Kind: core.SpecQualityModifier, Value: "imax"}},
		{"release type", core.CustomFormatSpec{Kind: core.SpecReleaseType, Value: "special"}},
	}
	for _, tt := range tests {
		f := core.CustomFormat{Name: "F", Specifications: []core.CustomFormatSpec{tt.spec}}
		if _, err := CustomFormatFromCore(f); !errors.Is(err, ErrUnknownValue) {
			t.Errorf("CustomFormatFromCore(unknown %s) error got = %v, want = %v", tt.name, err, ErrUnknownValue)
		}
	}
	bad := []Specification{
		{Kind: SpecKind(99)},
		{Kind: SpecLanguage, Value: -1},
		{Kind: SpecSource, Value: 42},
		{Kind: SpecQualityModifier, Value: 42},
		{Kind: SpecReleaseType, Value: 42},
	}
	for _, s := range bad {
		if _, err := CustomFormatToCore(CustomFormat{Name: "F", Specifications: []Specification{s}}); !errors.Is(err, ErrUnknownValue) {
			t.Errorf("CustomFormatToCore(%+v) error got = %v, want = %v", s, err, ErrUnknownValue)
		}
	}
}

func TestDelayProfileConversion(t *testing.T) {
	tests := []struct {
		name string
		p    core.DelayProfile
	}{
		{"default", core.DelayProfile{
			ID: tid(1), Default: true, Order: 0, EnableUsenet: true, EnableTorrent: true,
			PreferredProtocol: core.ProtocolUsenet,
		}},
		{"every field", core.DelayProfile{
			ID: tid(2), Order: 3, EnableTorrent: true, PreferredProtocol: core.ProtocolTorrent,
			UsenetDelay: time.Hour, TorrentDelay: 2 * time.Hour,
			BypassIfHighestQuality: true, BypassIfAboveCustomFormatScore: true, MinimumCustomFormatScore: 50,
			Tags: []string{"anime", "4k"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := DelayProfileFromCore(tt.p)
			if err != nil {
				t.Fatalf("DelayProfileFromCore() error = %v", err)
			}
			got, err := DelayProfileToCore(p)
			if err != nil {
				t.Fatalf("DelayProfileToCore() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.p) {
				t.Errorf("round trip got = %+v, want = %+v", got, tt.p)
			}
		})
	}
	if _, err := DelayProfileFromCore(core.DelayProfile{PreferredProtocol: "ftp"}); !errors.Is(err, ErrUnknownValue) {
		t.Errorf("DelayProfileFromCore(ftp) error got = %v, want = %v", err, ErrUnknownValue)
	}
	if _, err := DelayProfileToCore(DelayProfile{}); !errors.Is(err, ErrUnknownValue) {
		t.Errorf("DelayProfileToCore(unknown protocol) error got = %v, want = %v", err, ErrUnknownValue)
	}
}

func TestReleaseProfileConversion(t *testing.T) {
	tests := []core.ReleaseProfile{
		{ID: tid(1), Name: "All", Enabled: true, Required: []string{"x265"}},
		{
			ID: tid(2), Name: "Every field", Enabled: true, Required: []string{"/\\bHDR\\b/i"}, Ignored: []string{"CAM", "TS"},
			IndexerID: "torznab/2", Tags: []string{"4k"},
		},
	}
	for _, p := range tests {
		if got := ReleaseProfileToCore(ReleaseProfileFromCore(p)); !reflect.DeepEqual(got, p) {
			t.Errorf("round trip got = %+v, want = %+v", got, p)
		}
	}
}

func TestDefinitionConversion(t *testing.T) {
	tests := []struct {
		name       string
		d          core.QualityDefinition
		wantWeight int
		wantGroup  string
	}{
		{"movie", core.QualityDefinition{Kind: core.KindMovie, Quality: 3, Title: "WEB 1080", MinSize: 2, MaxSize: 100, PreferredSize: 50}, 18, "WEB 1080p"},
		{"series", core.QualityDefinition{Kind: core.KindSeries, Quality: 7, Title: "Bluray-1080p", MaxSize: 155, PreferredSize: 95}, 13, ""},
		{"series remux", core.QualityDefinition{Kind: core.KindSeries, Quality: 30, Title: "Remux-1080p", MinSize: 35}, 14, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := DefinitionFromCore(tt.d)
			if err != nil {
				t.Fatalf("DefinitionFromCore() error = %v", err)
			}
			if d.Weight != tt.wantWeight || d.GroupName != tt.wantGroup {
				t.Errorf("DefinitionFromCore() got = weight %d group %q, want = weight %d group %q", d.Weight, d.GroupName, tt.wantWeight, tt.wantGroup)
			}
			got, err := DefinitionToCore(d, tt.d.Kind)
			if err != nil {
				t.Fatalf("DefinitionToCore() error = %v", err)
			}
			if got != tt.d {
				t.Errorf("round trip got = %+v, want = %+v", got, tt.d)
			}
		})
	}
	errs := []core.QualityDefinition{
		{Kind: core.KindMovie, Quality: 99, Title: "x"},
		{Kind: core.KindSeries, Quality: 24, Title: "Workprint"},
		{Kind: core.KindEpisode, Quality: 1, Title: "SDTV"},
	}
	for _, d := range errs {
		if _, err := DefinitionFromCore(d); !errors.Is(err, ErrUnknownValue) {
			t.Errorf("DefinitionFromCore(%+v) error got = %v, want = %v", d, err, ErrUnknownValue)
		}
	}
	defs, err := DefinitionsFromCore(core.KindMovie, []core.QualityDefinition{tests[0].d})
	if err != nil {
		t.Fatalf("DefinitionsFromCore() error = %v", err)
	}
	if len(defs) != len(DefaultDefinitions(release.Movie)) {
		t.Errorf("len(DefinitionsFromCore()) got = %d, want = %d", len(defs), len(DefaultDefinitions(release.Movie)))
	}
	e := Env{Definitions: defs}
	if got := e.definition(release.WEBDL1080p); got.Title != "WEB 1080" || got.PreferredSize != 50 {
		t.Errorf("definition(WEBDL-1080p) got = %+v, want = the stored one", got)
	}
	if _, err := DefinitionsFromCore(core.KindSeries, []core.QualityDefinition{tests[0].d}); !errors.Is(err, ErrUnknownValue) {
		t.Errorf("DefinitionsFromCore(movie among series) error got = %v, want = %v", err, ErrUnknownValue)
	}
}

func TestConfigFromCore(t *testing.T) {
	s := core.DefaultAcquisitionSettings()
	s.Propers = core.PropersDoNotPrefer
	s.PreferIndexerFlags = true
	s.MaximumSize = 2048
	s.MinimumAge = 30
	s.Retention = 1000
	s.AllowHardcodedSubs = true
	s.WhitelistedHardcodedSubs = []string{"german"}
	s.SkipFreeSpaceCheck = true
	s.AvailabilityDelay = -3
	got, err := ConfigFromCore(s)
	if err != nil {
		t.Fatalf("ConfigFromCore() error = %v", err)
	}
	want := Config{
		Propers:                   PropersDoNotPrefer,
		PreferIndexerFlags:        true,
		CompletedDownloadHandling: true,
		MaximumSize:               2 << 30,
		MinimumAge:                30 * time.Minute,
		Retention:                 1000,
		AllowHardcodedSubs:        true,
		WhitelistedHardcodedSubs:  []string{"german"},
		SkipFreeSpaceCheck:        true,
		MinimumFreeSpace:          100 << 20,
		AvailabilityDelay:         -3,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ConfigFromCore() got = %+v, want = %+v", got, want)
	}
	s.Propers = "always"
	if _, err := ConfigFromCore(s); !errors.Is(err, ErrUnknownValue) {
		t.Errorf("ConfigFromCore(unknown propers) error got = %v, want = %v", err, ErrUnknownValue)
	}
}

func TestEnumConversions(t *testing.T) {
	for _, p := range core.Protocols {
		v, err := ProtocolFromCore(p)
		if err != nil {
			t.Fatalf("ProtocolFromCore(%q) error = %v", p, err)
		}
		if got, err := ProtocolToCore(v); got != p || err != nil {
			t.Errorf("ProtocolToCore(ProtocolFromCore(%q)) got = %q, %v, want = %q, nil", p, got, err, p)
		}
	}
	for _, p := range core.ProperDownloads {
		v, err := ProperDownloadFromCore(p)
		if err != nil {
			t.Fatalf("ProperDownloadFromCore(%q) error = %v", p, err)
		}
		if got, err := ProperDownloadToCore(v); got != p || err != nil {
			t.Errorf("ProperDownloadToCore(ProperDownloadFromCore(%q)) got = %q, %v, want = %q, nil", p, got, err, p)
		}
	}
	for _, k := range []core.ItemKind{core.KindMovie, core.KindSeries} {
		v, err := KindFromCore(k)
		if err != nil {
			t.Fatalf("KindFromCore(%q) error = %v", k, err)
		}
		if got, err := KindToCore(v); got != k || err != nil {
			t.Errorf("KindToCore(KindFromCore(%q)) got = %q, %v, want = %q, nil", k, got, err, k)
		}
	}
	if _, err := ProtocolFromCore(""); !errors.Is(err, ErrUnknownValue) {
		t.Errorf("ProtocolFromCore(\"\") error got = %v, want = %v", err, ErrUnknownValue)
	}
	if _, err := ProperDownloadToCore(ProperDownload(9)); !errors.Is(err, ErrUnknownValue) {
		t.Errorf("ProperDownloadToCore(9) error got = %v, want = %v", err, ErrUnknownValue)
	}
	if _, err := KindToCore(release.Kind(0)); !errors.Is(err, ErrUnknownValue) {
		t.Errorf("KindToCore(0) error got = %v, want = %v", err, ErrUnknownValue)
	}
}
