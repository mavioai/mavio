package subtitles

import (
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// The videos of Bazarr's test fixtures.
func dune() *Video {
	return &Video{
		Title: "Dune", AlternativeTitles: []string{"Dune: Part One"}, Year: 2021, Resolution: "1080p",
		Source: SourceWeb, AudioCodec: "Dolby Digital", VideoCodec: "H.264", ReleaseGroup: "SHITBOX",
	}
}

func breakingBad() *Video {
	return &Video{
		Episode: true, Series: "Breaking Bad", Season: 1, EpisodeNumber: 1, Source: SourceDiskHD,
		ReleaseGroup: "REWARD", Resolution: "720p", VideoCodec: "H.264",
	}
}

func lang(tag string) core.SubtitleLanguage { return core.SubtitleLanguage{Language: tag} }

func TestParseGuess(t *testing.T) {
	tests := []struct {
		name    string
		episode bool
		want    Guess
	}{
		{"Taxi Driver 1976 Bluray 720p x264.mkv", false, Guess{
			Titles: []string{"Taxi Driver"}, Year: 1976, Source: SourceDiskHD, Resolution: "720p", VideoCodec: "H.264",
		}},
		{"The Wire S01E01 Bluray 720p x264.mkv", true, Guess{
			Titles: []string{"The Wire"}, Season: 1, Episodes: []int{1}, Source: SourceDiskHD, Resolution: "720p", VideoCodec: "H.264",
		}},
		{"Better.Call.Saul.S06E04.720p.AMZN.WEB-DL.DDP5.1.H.264-NTb", true, Guess{
			Titles: []string{"Better Call Saul"}, Season: 6, Episodes: []int{4}, ReleaseGroup: "NTb", Source: SourceWeb,
			Resolution: "720p", VideoCodec: "H.264", AudioCodecs: []string{"Dolby Digital Plus"}, StreamingService: "Amazon Prime",
		}},
		{"Blade.Runner.1982.The.Final.Cut.1080p.BluRay.x264-SADPANDA", false, Guess{
			Titles: []string{"Blade Runner"}, Year: 1982, ReleaseGroup: "SADPANDA", Source: SourceDiskHD,
			Resolution: "1080p", VideoCodec: "H.264", Edition: "Final Cut",
		}},
	}
	for _, tt := range tests {
		got := ParseGuess(tt.name, tt.episode)
		if !guessEqual(got, tt.want) {
			t.Errorf("ParseGuess(%q): got = %+v, want = %+v", tt.name, got, tt.want)
		}
	}
}

func guessEqual(a, b Guess) bool {
	return slices.Equal(a.Titles, b.Titles) && a.EpisodeTitle == b.EpisodeTitle && a.Season == b.Season &&
		slices.Equal(a.Episodes, b.Episodes) && a.Year == b.Year && a.ReleaseGroup == b.ReleaseGroup &&
		a.Source == b.Source && a.Resolution == b.Resolution && a.VideoCodec == b.VideoCodec &&
		slices.Equal(a.AudioCodecs, b.AudioCodecs) && a.Edition == b.Edition && a.StreamingService == b.StreamingService
}

func TestGuessMatches(t *testing.T) {
	tests := []struct {
		name    string
		video   *Video
		release string
		want    []string
	}{
		{"same release", dune(), "Dune.2021.1080p.WEBRip.DD5.1.x264-SHITBOX", []string{
			MatchAudioCodec, MatchEdition, MatchReleaseGroup, MatchResolution, MatchSource,
			MatchStreamingService, MatchTitle, MatchVideoCodec, MatchYear,
		}},
		{"alternative title", dune(), "Dune Part One 2021 720p BluRay x264-SPARKS", []string{
			MatchEdition, MatchStreamingService, MatchTitle, MatchVideoCodec, MatchYear,
		}},
		{"group without source", dune(), "Dune.2021.1080p.BluRay.x264-SHITBOX", []string{
			MatchEdition, MatchResolution, MatchStreamingService, MatchTitle, MatchVideoCodec, MatchYear,
		}},
		{"episode", breakingBad(), "Breaking.Bad.S01E01.720p.BluRay.X264-REWARD", []string{
			MatchEdition, MatchEpisode, MatchReleaseGroup, MatchResolution, MatchSeason, MatchSeries,
			MatchSource, MatchStreamingService, MatchVideoCodec,
		}},
		{
			"equivalent group", &Video{Episode: true, Series: "Show", Season: 2, EpisodeNumber: 3, Source: SourceTV, ReleaseGroup: "LOL"},
			"Show.S02E03.HDTV.x264-DIMENSION",
			[]string{
				MatchEdition, MatchEpisode, MatchReleaseGroup, MatchSeason, MatchSeries, MatchSource, MatchStreamingService,
			},
		},
		{
			"multi-episode", &Video{Episode: true, Series: "Show", Season: 2, EpisodeNumber: 3},
			"Show.S02E03E04.720p.HDTV.x264-DIMENSION",
			[]string{
				MatchEdition, MatchEpisode, MatchSeason, MatchSeries, MatchStreamingService,
			},
		},
	}
	for _, tt := range tests {
		got := GuessMatches(tt.video, ParseGuess(tt.release, tt.video.Episode), false).Sorted()
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s: got = %v, want = %v", tt.name, got, tt.want)
		}
	}
}

func TestDefaultScores(t *testing.T) {
	if got := MaxScore(true); got != 360 {
		t.Errorf("MaxScore(episode): got = %d, want = 360", got)
	}
	if got := MaxScore(false); got != 180 {
		t.Errorf("MaxScore(movie): got = %d, want = 180", got)
	}
	// A hash is worth every other match but one point.
	for _, episode := range []bool{true, false} {
		s := ScoresFor(episode)
		if s[MatchHash] != s.Max()-1 {
			t.Errorf("hash score (episode %v): got = %d, want = %d", episode, s[MatchHash], s.Max()-1)
		}
	}
	s := &core.SubtitleSettings{MinimumScore: 90, MinimumScoreMovie: 70}
	if got := MinScore(s, true); got != 324 {
		t.Errorf("MinScore(episode): got = %d, want = 324", got)
	}
	if got := MinScore(s, false); got != 126 {
		t.Errorf("MinScore(movie): got = %d, want = 126", got)
	}
}

func TestScoreUnverifiableHash(t *testing.T) {
	for _, v := range []*Video{dune(), breakingBad()} {
		m := NewMatches(MatchHash, MatchTitle)
		score, without, final := Score(m, ScoreSubject{}, v, nil)
		want := ScoresFor(v.Episode)[MatchHash]
		if score != want || !slices.Equal(final.Sorted(), []string{MatchHash}) {
			t.Errorf("Score(episode %v): got = %d %v, want = %d [hash]", v.Episode, score, final.Sorted(), want)
		}
		if wantWithout := ScoresFor(v.Episode)[MatchTitle]; without != wantWithout {
			t.Errorf("score without hash (episode %v): got = %d, want = %d", v.Episode, without, wantWithout)
		}
		if !m.Has(MatchTitle) {
			t.Errorf("Score modified its matches: got = %v", m.Sorted())
		}
	}
}

func TestScore(t *testing.T) {
	yes, no := true, false
	special := breakingBad()
	special.Special = true
	tests := []struct {
		name      string
		video     *Video
		matches   Matches
		sub       ScoreSubject
		hi        *bool
		score, wo int
		final     []string
	}{
		{
			"valid hash", breakingBad(), NewMatches(MatchHash, MatchSeries, MatchSeason, MatchEpisode, MatchSource),
			ScoreSubject{HashVerifiable: true},
			nil, 359, 245,
			[]string{MatchHash},
		},
		{
			"invalid hash", breakingBad(), NewMatches(MatchHash, MatchSeries, MatchSeason, MatchEpisode),
			ScoreSubject{HashVerifiable: true},
			nil, 220, 220,
			[]string{MatchEpisode, MatchSeason, MatchSeries},
		},
		{
			"special's hash is not checked", special, NewMatches(MatchHash, MatchSeries),
			ScoreSubject{HashVerifiable: true},
			nil, 519, 160,
			[]string{MatchHash, MatchSeries},
		},
		{
			"movie hash", dune(), NewMatches(MatchHash, MatchVideoCodec, MatchSource),
			ScoreSubject{HashVerifiable: true},
			nil, 179, 31,
			[]string{MatchHash},
		},
		{"imdb id", dune(), NewMatches(MatchIMDbID), ScoreSubject{}, nil, 100, 100, []string{MatchIMDbID, MatchTitle, MatchYear}},
		{
			"series tvdb id", breakingBad(), NewMatches(MatchSeriesTVDBID, MatchSeason, MatchEpisode),
			ScoreSubject{},
			nil,
			310, 310,
			[]string{MatchEpisode, MatchSeason, MatchSeries, MatchSeriesTVDBID, MatchYear},
		},
		{"episode title", breakingBad(), NewMatches(MatchTitle), ScoreSubject{}, nil, 30, 30, []string{MatchEpisode, MatchTitle}},
		{
			"special by title", special, NewMatches(MatchTitle, MatchSeries, MatchYear),
			ScoreSubject{},
			nil,
			310, 310,
			[]string{MatchEpisode, MatchSeason, MatchSeries, MatchTitle, MatchYear},
		},
		{"wanted non-HI", dune(), NewMatches(MatchTitle), ScoreSubject{}, &no, 61, 61, []string{MatchHearingImpaired, MatchTitle}},
		{"unwanted HI", dune(), NewMatches(MatchTitle), ScoreSubject{HearingImpaired: true}, &no, 60, 60, []string{MatchTitle}},
		{"wanted HI", dune(), NewMatches(MatchTitle), ScoreSubject{HearingImpaired: true}, &yes, 61, 61, []string{MatchHearingImpaired, MatchTitle}},
	}
	for _, tt := range tests {
		score, wo, final := Score(tt.matches, tt.sub, tt.video, tt.hi)
		if score != tt.score || wo != tt.wo || !slices.Equal(final.Sorted(), tt.final) {
			t.Errorf("%s: got = %d %d %v, want = %d %d %v", tt.name, score, wo, final.Sorted(), tt.score, tt.wo, tt.final)
		}
	}
}

func TestLanguageEquals(t *testing.T) {
	es, mx := lang("es"), lang("es-MX")
	forced := core.SubtitleLanguage{Language: "es", Forced: true}
	tests := []struct {
		name   string
		equals LanguageEquals
		in     []core.SubtitleLanguage
		want   []core.SubtitleLanguage
	}{
		{"empty", nil, []core.SubtitleLanguage{es}, []core.SubtitleLanguage{es}},
		{"adds the equal", LanguageEquals{{From: es, To: mx}}, []core.SubtitleLanguage{es}, []core.SubtitleLanguage{es, mx}},
		{"nothing equal", LanguageEquals{{From: lang("en"), To: es}}, []core.SubtitleLanguage{es}, []core.SubtitleLanguage{es}},
		{"forced differs", LanguageEquals{{From: forced, To: mx}}, []core.SubtitleLanguage{es}, []core.SubtitleLanguage{es}},
	}
	for _, tt := range tests {
		if got := tt.equals.CheckSet(tt.in); !sameLanguages(got, tt.want) {
			t.Errorf("CheckSet %s: got = %v, want = %v", tt.name, got, tt.want)
		}
	}
	eq := LanguageEquals{{From: es, To: mx}}
	if got := eq.Translate([]core.SubtitleLanguage{mx}); !sameLanguages(got, []core.SubtitleLanguage{mx, es}) {
		t.Errorf("Translate: got = %v, want = [es-MX es]", got)
	}
	if got := eq.Apply(es); got != mx {
		t.Errorf("Apply(es): got = %v, want = es-MX", got)
	}
	if got := eq.Apply(lang("en")); got != lang("en") {
		t.Errorf("Apply(en): got = %v, want = en", got)
	}
}

func TestFilterLanguageEquals(t *testing.T) {
	es, mx, en := lang("es"), lang("es-MX"), lang("en")
	cands := []Candidate{
		{Provider: "opensubtitlescom", ID: "1", Language: es, Matches: NewMatches(MatchTitle, MatchYear)},
		{Provider: "opensubtitlescom", ID: "2", Language: es, Matches: NewMatches(MatchTitle)},
	}
	langsOf := func(cs []Candidate) []core.SubtitleLanguage {
		var out []core.SubtitleLanguage
		for _, c := range cs {
			out = append(out, c.Language)
		}
		return out
	}
	if got := langsOf(Filter(cands, LanguageEquals{{From: es, To: mx}}, nil, nil)); !slices.Equal(got, []core.SubtitleLanguage{mx, mx}) {
		t.Errorf("Filter es→es-MX: got = %v, want = [es-MX es-MX]", got)
	}
	if got := langsOf(Filter(cands, LanguageEquals{{From: mx, To: es}}, nil, nil)); !slices.Equal(got, []core.SubtitleLanguage{es, es}) {
		t.Errorf("Filter es-MX→es: got = %v, want = [es es]", got)
	}
	if got := langsOf(Filter(cands, nil, nil, nil)); !slices.Equal(got, []core.SubtitleLanguage{es, es}) {
		t.Errorf("Filter: got = %v, want = [es es]", got)
	}
	// Subtitles all counting as English serve no Spanish request.
	filtered := Filter(cands, LanguageEquals{{From: mx, To: en}, {From: es, To: en}}, nil, nil)
	if got := Choose(filtered, dune(), []core.SubtitleLanguage{es}, HIAllowed, 0); len(got) != 0 {
		t.Errorf("Choose: got = %d candidates, want = 0", len(got))
	}
}

func TestFilter(t *testing.T) {
	ban, err := NewBanList(&core.SubtitleProfile{Name: "p", MustContain: []string{"bluray"}, MustNotContain: []string{"hc"}})
	if err != nil {
		t.Fatal(err)
	}
	cands := []Candidate{
		{Provider: "a", ID: "1", ReleaseInfo: "Dune.2021.BluRay"},
		{Provider: "a", ID: "1", ReleaseInfo: "Dune.2021.BluRay"},
		{Provider: "a", ID: "2", ReleaseInfo: "Dune.2021.WEB"},
		{Provider: "a", ID: "3", ReleaseInfo: "Dune.2021.BluRay.HC"},
		{Provider: "a", ID: "4"},
		{Provider: "b", ID: "1", ReleaseInfo: "Dune.2021.bluray"},
		{Provider: "a", ID: "5", ReleaseInfo: "Dune.2021.BluRay"},
	}
	blocked := []core.SubtitleBlocklistEntry{{Provider: "a", SubtitleID: "5"}}
	var got []string
	for _, c := range Filter(cands, nil, ban, blocked) {
		got = append(got, c.Provider+c.ID)
	}
	if want := []string{"a1", "a4", "b1"}; !slices.Equal(got, want) {
		t.Errorf("Filter: got = %v, want = %v", got, want)
	}
	if _, err := NewBanList(&core.SubtitleProfile{Name: "p", MustContain: []string{"("}}); err == nil {
		t.Error("NewBanList: got = nil, want = an error")
	}
}

func TestChoose(t *testing.T) {
	en := lang("en")
	enHI := core.SubtitleLanguage{Language: "en", HearingImpaired: true}
	full := NewMatches(MatchSeries, MatchSeason, MatchEpisode, MatchSource, MatchReleaseGroup)
	cands := []Candidate{
		{ID: "low", Language: en, Matches: NewMatches(MatchSeries, MatchSeason, MatchEpisode)},
		{ID: "best", Language: en, HearingImpairedVerifiable: true, Matches: full},
		{ID: "hi", Language: enHI, HearingImpairedVerifiable: true, Matches: full},
		{ID: "other series", Language: en, Matches: NewMatches(MatchSeason, MatchEpisode, MatchSource, MatchYear, MatchReleaseGroup)},
		{ID: "imdb", Language: en, Matches: NewMatches(MatchIMDbID, MatchSeason, MatchEpisode)},
		{ID: "unverifiable hash", Language: en, Matches: NewMatches(MatchHash)},
		{ID: "french", Language: lang("fr"), Matches: full},
	}
	ids := func(rs []Ranked) []string {
		var out []string
		for _, r := range rs {
			out = append(out, r.ID)
		}
		return out
	}
	tests := []struct {
		name string
		mode HIMode
		min  int
		want []string
	}{
		{"allowed", HIAllowed, 0, []string{"unverifiable hash", "imdb", "best", "hi", "low"}},
		{"force non-HI", HIForceNon, 0, []string{"unverifiable hash", "imdb", "best", "low"}},
		// Subtitles whose kind the provider does not know are kept.
		{"force HI", HIForce, 0, []string{"unverifiable hash", "imdb", "hi", "low"}},
		{"minimum", HIAllowed, 300, []string{"unverifiable hash", "imdb"}},
	}
	for _, tt := range tests {
		if got := ids(Choose(cands, breakingBad(), []core.SubtitleLanguage{en}, tt.mode, tt.min)); !slices.Equal(got, tt.want) {
			t.Errorf("%s: got = %v, want = %v", tt.name, got, tt.want)
		}
	}
}

func TestHIModeFor(t *testing.T) {
	p := &core.SubtitleProfile{Items: []core.SubtitleProfileItem{
		{Language: "en", HI: core.SubtitleHIRequired},
		{Language: "en", Forced: true},
		{Language: "fr", HI: core.SubtitleHIExcluded},
	}}
	tests := []struct {
		l    core.SubtitleLanguage
		want HIMode
	}{
		{lang("en"), HIForce},
		{core.SubtitleLanguage{Language: "en", Forced: true}, HIAllowed},
		{lang("fr"), HIForceNon},
		{lang("de"), HIAllowed},
	}
	for _, tt := range tests {
		if got := HIModeFor(p, tt.l); got != tt.want {
			t.Errorf("HIModeFor(%v): got = %v, want = %v", tt.l, got, tt.want)
		}
	}
}

func TestMissing(t *testing.T) {
	en, fr := lang("en"), lang("fr")
	enHI := core.SubtitleLanguage{Language: "en", HearingImpaired: true}
	enForced := core.SubtitleLanguage{Language: "en", Forced: true}
	item := func(id int, l string, hi core.SubtitleHI) core.SubtitleProfileItem {
		return core.SubtitleProfileItem{ID: id, Language: l, HI: hi}
	}
	tests := []struct {
		name     string
		profile  core.SubtitleProfile
		existing []core.SubtitleLanguage
		audio    []string
		equals   LanguageEquals
		want     []core.SubtitleLanguage
	}{
		{
			"none existing",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{item(1, "en", ""), item(2, "fr", "")}},
			nil, nil, nil,
			[]core.SubtitleLanguage{en, fr},
		},
		{
			"one existing",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{item(1, "en", ""), item(2, "fr", "")}},
			[]core.SubtitleLanguage{fr},
			nil, nil,
			[]core.SubtitleLanguage{en},
		},
		{
			"HI serves normal",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{item(1, "en", "")}},
			[]core.SubtitleLanguage{enHI},
			nil, nil, nil,
		},
		{
			"HI excluded",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{item(1, "en", core.SubtitleHIExcluded)}},
			[]core.SubtitleLanguage{enHI},
			nil, nil,
			[]core.SubtitleLanguage{en},
		},
		{
			"non-HI serves excluded",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{item(1, "en", core.SubtitleHIExcluded)}},
			[]core.SubtitleLanguage{en},
			nil, nil, nil,
		},
		{
			"HI required",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{item(1, "en", core.SubtitleHIRequired)}},
			[]core.SubtitleLanguage{en},
			nil, nil,
			[]core.SubtitleLanguage{enHI},
		},
		{
			"forced",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{{ID: 1, Language: "en", Forced: true}}},
			[]core.SubtitleLanguage{en},
			nil, nil,
			[]core.SubtitleLanguage{enForced},
		},
		{
			"cutoff",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{item(1, "en", ""), item(2, "fr", "")}, Cutoff: 2},
			[]core.SubtitleLanguage{fr},
			nil, nil, nil,
		},
		{
			"cutoff by any",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{item(1, "en", ""), item(2, "fr", "")}, Cutoff: core.SubtitleCutoffAny},
			[]core.SubtitleLanguage{en},
			nil, nil, nil,
		},
		{
			"cutoff by HI",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{item(1, "en", ""), item(2, "fr", "")}, Cutoff: 1},
			[]core.SubtitleLanguage{enHI},
			nil, nil, nil,
		},
		{
			"cutoff unmet",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{item(1, "en", ""), item(2, "fr", "")}, Cutoff: 1},
			[]core.SubtitleLanguage{fr},
			nil, nil,
			[]core.SubtitleLanguage{en},
		},
		{
			"audio excludes",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{{ID: 1, Language: "en", AudioExclude: true}, item(2, "fr", "")}},
			nil,
			[]string{"EN"},
			nil,
			[]core.SubtitleLanguage{fr},
		},
		{
			"audio excluded meets cutoff",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{{ID: 1, Language: "en", AudioExclude: true}, item(2, "fr", "")}, Cutoff: 1},
			nil,
			[]string{"en"},
			nil, nil,
		},
		{
			"audio only includes",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{{ID: 1, Language: "en", AudioOnlyInclude: true}, item(2, "fr", "")}},
			nil,
			[]string{"fr"},
			nil,
			[]core.SubtitleLanguage{fr},
		},
		{
			"audio included",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{{ID: 1, Language: "en", AudioOnlyInclude: true}}},
			nil,
			[]string{"en"},
			nil,
			[]core.SubtitleLanguage{en},
		},
		{
			"equal language",
			core.SubtitleProfile{Items: []core.SubtitleProfileItem{item(1, "es-MX", "")}},
			[]core.SubtitleLanguage{lang("es")},
			nil,
			LanguageEquals{{From: lang("es"), To: lang("es-MX")}},
			nil,
		},
	}
	for _, tt := range tests {
		if got := Missing(&tt.profile, tt.existing, tt.audio, tt.equals); !slices.Equal(got, tt.want) {
			t.Errorf("%s: got = %v, want = %v", tt.name, got, tt.want)
		}
	}
}

func TestUpgradable(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	day := func(n int) time.Time { return now.AddDate(0, 0, -n) }
	id := func(b byte) core.ID { return core.ID{b} }
	en, fr := lang("en"), lang("fr")
	event := func(b byte, a core.SubtitleAction, path string, l core.SubtitleLanguage, date time.Time, score int) core.SubtitleHistoryEvent {
		return core.SubtitleHistoryEvent{ID: id(b), Action: a, VideoPath: path, Language: l, Date: date, Score: score, MaxScore: 360}
	}
	upgraded := event(7, core.SubtitleUpgraded, "/c.mkv", en, day(1), 300)
	upgraded.UpgradedFrom = id(6)
	events := []core.SubtitleHistoryEvent{
		event(1, core.SubtitleDownloaded, "/a.mkv", en, day(3), 300),
		event(2, core.SubtitleDownloaded, "/a.mkv", fr, day(2), 357),
		event(3, core.SubtitleDownloaded, "/b.mkv", en, day(4), 300),
		event(4, core.SubtitleDeleted, "/b.mkv", en, day(3), 0),
		event(5, core.SubtitleDownloaded, "/old.mkv", en, day(30), 300),
		event(6, core.SubtitleDownloaded, "/c.mkv", en, day(2), 290),
		upgraded,
		event(8, core.SubtitleManual, "/d.mkv", en, day(1), 100),
		event(9, core.SubtitleDownloaded, "/e.mkv", en, day(5), 100),
		event(10, core.SubtitleSynced, "/e.mkv", en, day(4), 100),
	}
	s := &core.SubtitleSettings{Upgrade: true, UpgradeDays: 7}
	got := func() []string {
		var out []string
		for _, u := range Upgradable(events, s, now) {
			out = append(out, string('0'+rune(u.Event.ID[0]))+"←"+string('0'+rune(u.Root[0])))
		}
		return out
	}
	if g, want := got(), []string{"7←6", "1←1", "9←9"}; !slices.Equal(g, want) {
		t.Errorf("Upgradable: got = %v, want = %v", g, want)
	}
	s.UpgradeManual = true
	if g, want := got(), []string{"7←6", "8←8", "1←1", "9←9"}; !slices.Equal(g, want) {
		t.Errorf("Upgradable with manual: got = %v, want = %v", g, want)
	}
	s.Upgrade = false
	if g := got(); len(g) != 0 {
		t.Errorf("Upgradable disabled: got = %v, want = []", g)
	}
	if got := UpgradeMinScore(events[0]); got != 301 {
		t.Errorf("UpgradeMinScore: got = %d, want = 301", got)
	}
}

func TestStillDesired(t *testing.T) {
	p := &core.SubtitleProfile{Items: []core.SubtitleProfileItem{
		{Language: "en"},
		{Language: "fr", HI: core.SubtitleHIRequired},
		{Language: "de", HI: core.SubtitleHIExcluded},
		{Language: "es", Forced: true},
	}}
	tests := []struct {
		l    core.SubtitleLanguage
		want bool
	}{
		{lang("en"), true},
		{core.SubtitleLanguage{Language: "en", HearingImpaired: true}, true},
		{core.SubtitleLanguage{Language: "en", Forced: true}, false},
		{lang("fr"), false},
		{core.SubtitleLanguage{Language: "fr", HearingImpaired: true}, true},
		{lang("de"), true},
		{core.SubtitleLanguage{Language: "de", HearingImpaired: true}, false},
		{lang("es"), false},
		{core.SubtitleLanguage{Language: "es", Forced: true}, true},
		{lang("it"), false},
	}
	for _, tt := range tests {
		if got := StillDesired(p, tt.l); got != tt.want {
			t.Errorf("StillDesired(%v): got = %v, want = %v", tt.l, got, tt.want)
		}
	}
	enHI := core.SubtitleLanguage{Language: "en", HearingImpaired: true}
	if got := UpgradeLanguage(p, enHI); got != lang("en") {
		t.Errorf("UpgradeLanguage(en:hi): got = %v, want = en", got)
	}
	frHI := core.SubtitleLanguage{Language: "fr", HearingImpaired: true}
	if got := UpgradeLanguage(p, frHI); got != frHI {
		t.Errorf("UpgradeLanguage(fr:hi): got = %v, want = fr:hi", got)
	}
}

func TestSearchActive(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	s := &core.SubtitleSettings{AdaptiveSearching: true, AdaptiveDelay: 21 * 24 * time.Hour, AdaptiveDelta: 7 * 24 * time.Hour}
	at := func(first, last int) *core.SubtitleSearchAttempt {
		return &core.SubtitleSearchAttempt{First: now.AddDate(0, 0, -first), Last: now.AddDate(0, 0, -last)}
	}
	tests := []struct {
		name    string
		attempt *core.SubtitleSearchAttempt
		want    bool
	}{
		{"first search", nil, true},
		{"within the delay", at(10, 1), true},
		{"after the delay, searched recently", at(30, 1), false},
		{"after the delay, a delta ago", at(30, 7), true},
	}
	for _, tt := range tests {
		if got := SearchActive(s, tt.attempt, now); got != tt.want {
			t.Errorf("%s: got = %v, want = %v", tt.name, got, tt.want)
		}
	}
	if !SearchActive(&core.SubtitleSettings{}, at(30, 1), now) {
		t.Error("disabled: got = false, want = true")
	}
	a := RecordAttempt(nil, core.ID{1}, lang("en"), now)
	a = RecordAttempt(&a, core.ID{1}, lang("en"), now.Add(time.Hour))
	if !a.First.Equal(now) || !a.Last.Equal(now.Add(time.Hour)) {
		t.Errorf("RecordAttempt: got = %v–%v, want = %v–%v", a.First, a.Last, now, now.Add(time.Hour))
	}
}

func TestProfileFor(t *testing.T) {
	profiles := []core.SubtitleProfile{
		{ID: core.ID{1}, Name: "a"},
		{ID: core.ID{2}, Name: "b", Tag: "anime"},
		{ID: core.ID{3}, Name: "c", Tag: "kids"},
	}
	tests := []struct {
		name     string
		assigned core.ID
		tags     []string
		useTags  bool
		want     core.ID
	}{
		{"assigned", core.ID{1}, nil, true, core.ID{1}},
		{"none", core.ID{}, nil, true, core.ID{}},
		{"tag", core.ID{1}, []string{"Kids", "anime"}, true, core.ID{2}},
		{"tags unused", core.ID{1}, []string{"anime"}, false, core.ID{1}},
		{"no-profile tag", core.ID{1}, []string{"anime", "skip"}, true, core.ID{}},
		{"unknown", core.ID{9}, nil, true, core.ID{}},
	}
	for _, tt := range tests {
		var got core.ID
		if p := ProfileFor(profiles, tt.assigned, tt.tags, tt.useTags, []string{"skip"}); p != nil {
			got = p.ID
		}
		if got != tt.want {
			t.Errorf("%s: got = %v, want = %v", tt.name, got, tt.want)
		}
	}
}
