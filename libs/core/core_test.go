package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNewIDFormat(t *testing.T) {
	before := time.Now().Truncate(time.Millisecond)
	id := NewID()
	after := time.Now()

	if v := id[6] >> 4; v != 7 {
		t.Errorf("version = %d, want 7", v)
	}
	if variant := id[8] >> 6; variant != 0b10 {
		t.Errorf("variant bits = %02b, want 10", variant)
	}
	if ts := id.Time(); ts.Before(before) || ts.After(after) {
		t.Errorf("Time() = %v, want between %v and %v", ts, before, after)
	}

	parsed, err := ParseID(id.String())
	if err != nil || parsed != id {
		t.Errorf("ParseID(String()) = %v, %v; want %v", parsed, err, id)
	}
}

func TestNewIDMonotonic(t *testing.T) {
	prev := NewID()
	for range 10000 {
		next := NewID()
		if bytes.Compare(next[:], prev[:]) <= 0 {
			t.Fatalf("NewID not increasing: %s then %s", prev, next)
		}
		prev = next
	}
}

func TestParseIDErrors(t *testing.T) {
	for _, s := range []string{
		"",
		"0190f5a4-3b2c-7d1e-8f00-12345678901",  // too short
		"0190f5a43b2c7d1e8f0012345678901234ab", // no dashes
		"0190f5a4-3b2c-7d1e-8f00-12345678901z", // not hex
	} {
		if _, err := ParseID(s); !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseID(%q) error = %v, want ErrInvalid", s, err)
		}
	}
}

func TestIDJSON(t *testing.T) {
	id := MustParseID("0190f5a4-3b2c-7d1e-8f00-123456789012")
	b, err := json.Marshal(struct{ ID ID }{id})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"ID":"0190f5a4-3b2c-7d1e-8f00-123456789012"}`; string(b) != want {
		t.Errorf("Marshal = %s, want %s", b, want)
	}
	var out struct{ ID ID }
	if err := json.Unmarshal(b, &out); err != nil || out.ID != id {
		t.Errorf("Unmarshal = %v, %v; want %v", out.ID, err, id)
	}
}

func TestEnumsValid(t *testing.T) {
	for _, k := range ItemKinds {
		if !k.Valid() {
			t.Errorf("ItemKind %q not valid", k)
		}
	}
	if ItemKind("tvshow").Valid() || LibraryKind("").Valid() || StreamKind("lyric").Valid() {
		t.Error("unknown kinds reported valid")
	}
	if !KindSeries.IsContainer() || KindEpisode.IsContainer() {
		t.Error("IsContainer wrong for series/episode")
	}
	if !KindTrack.HasMedia() || KindMusicAlbum.HasMedia() {
		t.Error("HasMedia wrong for track/album")
	}
}

func validItem() Item {
	return Item{ID: NewID(), LibraryID: NewID(), Kind: KindMovie, Name: "Heat"}
}

func TestValidate(t *testing.T) {
	ten := 10.5
	tests := []struct {
		name string
		err  error
	}{
		{"valid item", ptr(validItem()).Validate()},
		{"valid library", (&Library{Name: "Movies", Kind: LibraryMovies, Paths: []string{"/media"}}).Validate()},
		{"valid user", (&User{ID: NewID(), Name: "alice", PasswordHash: "$argon2id$x"}).Validate()},
		{"valid job", (&Job{ID: NewID(), Kind: "library.scan", MaxAttempts: 3}).Validate()},
		{"valid session", (&AuthSession{ID: NewID(), UserID: NewID(), TokenHash: make([]byte, 32), DeviceID: "d"}).Validate()},
	}
	for _, tt := range tests {
		if tt.err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, tt.err)
		}
	}

	invalid := []struct {
		name string
		err  error
	}{
		{"item without name", func() error { it := validItem(); it.Name = ""; return it.Validate() }()},
		{"item unknown kind", func() error { it := validItem(); it.Kind = "show"; return it.Validate() }()},
		{"item own parent", func() error { it := validItem(); it.ParentID = it.ID; return it.Validate() }()},
		{"extra without owner", func() error { it := validItem(); it.Extra = ExtraTrailer; return it.Validate() }()},
		{"item rating range", func() error { it := validItem(); it.CommunityRating = 11; return it.Validate() }()},
		{"item 3D format", func() error { it := validItem(); it.Video3DFormat = "sbs"; return it.Validate() }()},
		{"item locked field", func() error { it := validItem(); it.LockedFields = []MetadataField{"plot"}; return it.Validate() }()},
		{"item air day", func() error { it := validItem(); it.AirDays = []time.Weekday{7}; return it.Validate() }()},
		{"library without paths", (&Library{Name: "x", Kind: LibraryMovies}).Validate()},
		{"library unknown kind", (&Library{Name: "x", Kind: "tv", Paths: []string{"/"}}).Validate()},
		{"user without credentials", (&User{ID: NewID(), Name: "bob"}).Validate()},
		{"user bad subtitle mode", (&User{ID: NewID(), Name: "bob", AuthProvider: "ldap", Preferences: UserPreferences{SubtitleMode: "sometimes"}}).Validate()},
		{"user data rating", (&UserData{UserID: NewID(), ItemID: NewID(), Rating: &ten}).Validate()},
		{"user data position", (&UserData{UserID: NewID(), ItemID: NewID(), Position: -time.Second}).Validate()},
		{"job without attempts", (&Job{ID: NewID(), Kind: "x"}).Validate()},
		{"session without device", (&AuthSession{ID: NewID(), UserID: NewID(), TokenHash: make([]byte, 32)}).Validate()},
		{"session token", (&AuthSession{ID: NewID(), UserID: NewID(), TokenHash: []byte("token"), DeviceID: "d"}).Validate()},
		{"image without location", (&Image{ID: NewID(), OwnerID: NewID(), Kind: ImagePrimary}).Validate()},
		{"credit unknown kind", (&Credit{ItemID: NewID(), PersonID: NewID(), Kind: "gaffer"}).Validate()},
		{"source bad stream", (&MediaSource{ID: NewID(), ItemID: NewID(), Path: "/a.mkv", Streams: []MediaStream{{Kind: "lyric"}}}).Validate()},
	}
	for _, tt := range invalid {
		if !errors.Is(tt.err, ErrInvalid) {
			t.Errorf("%s: error = %v, want ErrInvalid", tt.name, tt.err)
		}
	}
}

func ptr[T any](v T) *T { return &v }

func TestItemQueryValidate(t *testing.T) {
	yes := true
	tests := []struct {
		name  string
		q     ItemQuery
		valid bool
	}{
		{"empty", ItemQuery{}, true},
		{"recursive with parent", ItemQuery{ParentID: NewID(), Recursive: true}, true},
		{"recursive without parent", ItemQuery{Recursive: true}, false},
		{"limit too large", ItemQuery{Limit: MaxPageSize + 1}, false},
		{"negative offset", ItemQuery{Offset: -1}, false},
		{"empty year range", ItemQuery{YearFrom: 2020, YearTo: 2010}, false},
		{"played without user", ItemQuery{Played: &yes}, false},
		{"played with user", ItemQuery{UserID: NewID(), Played: &yes}, true},
		{"unknown kind", ItemQuery{Kinds: []ItemKind{"show"}}, false},
		{"unknown sort", ItemQuery{Sort: []SortSpec{{Field: "title"}}}, false},
		{"user sort without user", ItemQuery{Sort: []SortSpec{{Field: SortLastPlayed}}}, false},
	}
	for _, tt := range tests {
		err := tt.q.Validate()
		if (err == nil) != tt.valid {
			t.Errorf("%s: Validate() = %v, want valid=%v", tt.name, err, tt.valid)
		}
		if err != nil && !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: error %v does not wrap ErrInvalid", tt.name, err)
		}
	}
	if got := (&ItemQuery{}).PageSize(); got != MaxPageSize {
		t.Errorf("PageSize() = %d, want %d", got, MaxPageSize)
	}
}

func TestRetryDelay(t *testing.T) {
	got := []time.Duration{}
	for n := 0; n <= 9; n++ {
		got = append(got, RetryDelay(n))
	}
	want := []time.Duration{
		30 * time.Second, 30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute,
		8 * time.Minute, 16 * time.Minute, 32 * time.Minute, time.Hour, time.Hour,
	}
	if !slices.Equal(got, want) {
		t.Errorf("RetryDelay(0..9) = %v, want %v", got, want)
	}
}

func TestMediaSourceHelpers(t *testing.T) {
	src := MediaSource{Streams: []MediaStream{
		{Index: 0, Kind: StreamVideo}, {Index: 1, Kind: StreamAudio}, {Index: 2, Kind: StreamAudio},
	}}
	if got := src.StreamsOf(StreamAudio); len(got) != 2 || got[0].Index != 1 {
		t.Errorf("StreamsOf(audio) = %+v", got)
	}
	if r := (Rational{24000, 1001}); r.String() != "24000/1001" || r.Float() < 23.97 || r.Float() > 23.98 {
		t.Errorf("Rational = %s (%v)", r, r.Float())
	}
	if !(Rational{}).IsZero() || (Rational{}).Float() != 0 {
		t.Error("zero Rational")
	}
}

func TestUserPolicyLibraries(t *testing.T) {
	a, b := NewID(), NewID()
	if !(&UserPolicy{}).CanAccessLibrary(a) {
		t.Error("nil Libraries should allow all")
	}
	p := UserPolicy{Libraries: []ID{a}}
	if !p.CanAccessLibrary(a) || p.CanAccessLibrary(b) {
		t.Error("explicit Libraries not enforced")
	}
}

func TestUserPolicyCanAccess(t *testing.T) {
	lib, other := NewID(), NewID()
	tests := []struct {
		name   string
		policy UserPolicy
		item   Item
		want   bool
	}{
		{"unrestricted", UserPolicy{}, Item{LibraryID: lib, ParentalRating: new(18)}, true},
		{"other library", UserPolicy{Libraries: []ID{lib}}, Item{LibraryID: other}, false},
		{"within rating", UserPolicy{MaxParentalRating: new(12)}, Item{LibraryID: lib, ParentalRating: new(12)}, true},
		{"above rating", UserPolicy{MaxParentalRating: new(12)}, Item{LibraryID: lib, ParentalRating: new(13)}, false},
		{"unrated", UserPolicy{MaxParentalRating: new(12)}, Item{LibraryID: lib}, true},
		{"unrated blocked", UserPolicy{MaxParentalRating: new(12), BlockUnrated: true}, Item{LibraryID: lib}, false},
		{"block without maximum", UserPolicy{BlockUnrated: true}, Item{LibraryID: lib}, true},
		{"inherited above rating", UserPolicy{MaxParentalRating: new(12)}, Item{LibraryID: lib, InheritedRating: new(17)}, false},
		{"inherited within rating", UserPolicy{MaxParentalRating: new(12), BlockUnrated: true}, Item{LibraryID: lib, InheritedRating: new(7)}, true},
		{"own rating over inherited", UserPolicy{MaxParentalRating: new(12)}, Item{LibraryID: lib, ParentalRating: new(7), InheritedRating: new(17)}, true},
		{"all ages only", UserPolicy{MaxParentalRating: new(0), BlockUnrated: true}, Item{LibraryID: lib, ParentalRating: new(0)}, true},
		{"rated for all ages is rated", UserPolicy{MaxParentalRating: new(0), BlockUnrated: true}, Item{LibraryID: lib, ParentalRating: new(7)}, false},
	}
	for _, tt := range tests {
		if got := tt.policy.CanAccess(&tt.item); got != tt.want {
			t.Errorf("%s: CanAccess = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIDSQL(t *testing.T) {
	id := NewID()
	v, err := id.Value()
	if err != nil || v != id.String() {
		t.Fatalf("Value() = %v, %v", v, err)
	}
	for _, src := range []any{id.String(), []byte(id.String()), id[:]} {
		var got ID
		if err := got.Scan(src); err != nil || got != id {
			t.Errorf("Scan(%T) = %v, %v; want %v", src, got, err, id)
		}
	}
	var got ID
	if err := got.Scan(nil); err != nil || !got.IsZero() {
		t.Errorf("Scan(nil) = %v, %v", got, err)
	}
	if err := got.Scan(42); !errors.Is(err, ErrInvalid) {
		t.Errorf("Scan(int) error = %v, want ErrInvalid", err)
	}
}

func TestValueAndPersonQueries(t *testing.T) {
	if err := (&ValueQuery{Kind: ValueGenre, Search: "dra"}).Validate(); err != nil {
		t.Errorf("valid value query: %v", err)
	}
	if err := (&ValueQuery{Kind: "mood"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown kind: %v", err)
	}
	if err := (&ValueQuery{Kind: ValueTag, Limit: MaxPageSize + 1}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Errorf("limit: %v", err)
	}
	if err := (&PersonQuery{Limit: -1}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Errorf("person limit: %v", err)
	}
	invalid := []struct {
		name string
		err  error
	}{
		{"searched years", (&ValueQuery{Kind: ValueYear, Search: "19"}).Validate()},
		{"negative offset", (&ValueQuery{Kind: ValueGenre, Offset: -1}).Validate()},
		{"unknown item kind", (&ValueQuery{Kind: ValueGenre, Items: ItemFilter{Kinds: []ItemKind{"film"}}}).Validate()},
		{"unknown credit kind", (&PersonQuery{CreditKinds: []CreditKind{"grip"}}).Validate()},
		{"person offset", (&PersonQuery{Offset: -1}).Validate()},
	}
	for _, tt := range invalid {
		if !errors.Is(tt.err, ErrInvalid) {
			t.Errorf("%s: got = %v, want = ErrInvalid", tt.name, tt.err)
		}
	}
}

func TestVideoRangeType(t *testing.T) {
	pq := MediaStream{Kind: StreamVideo, ColorTransfer: "smpte2084", ColorPrimaries: "bt2020", ColorSpace: "bt2020nc"}
	dv := func(s MediaStream, profile, compat int) MediaStream {
		s.DolbyVision = &DolbyVision{Profile: profile, BLCompatibilityID: compat, RPUPresent: true, BLPresent: true}
		return s
	}
	plus := pq
	plus.HDR10Plus = true
	hlg := MediaStream{Kind: StreamVideo, ColorTransfer: "arib-std-b67", ColorPrimaries: "bt2020", ColorSpace: "bt2020nc"}
	sdr := MediaStream{Kind: StreamVideo}
	tests := []struct {
		name  string
		s     MediaStream
		rng   VideoRange
		rtype VideoRangeType
	}{
		{"sdr", sdr, RangeSDR, RangeTypeSDR},
		{"hdr10", pq, RangeHDR, RangeTypeHDR10},
		{"hdr10+", plus, RangeHDR, RangeTypeHDR10Plus},
		{"hlg", hlg, RangeHDR, RangeTypeHLG},
		{"dv 5", dv(sdr, 5, 0), RangeHDR, RangeTypeDOVI},
		{"dv 8.1", dv(pq, 8, 1), RangeHDR, RangeTypeDOVIWithHDR10},
		{"dv 8.1 with hdr10+", dv(plus, 8, 1), RangeHDR, RangeTypeDOVIWithHDR10Plus},
		{"dv 8.4", dv(hlg, 8, 4), RangeHDR, RangeTypeDOVIWithHLG},
		{"dv 8.2", dv(sdr, 8, 2), RangeSDR, RangeTypeDOVIWithSDR},
		{"dv 7", dv(pq, 7, 6), RangeHDR, RangeTypeDOVIWithEL},
		{"dv 8.1 without pq", dv(sdr, 8, 1), RangeSDR, RangeTypeDOVIInvalid},
		{"dv 8.0", dv(pq, 8, 0), RangeHDR, RangeTypeDOVIInvalid},
		{"dovi tag", MediaStream{Kind: StreamVideo, CodecTag: "dvh1"}, RangeSDR, RangeTypeSDR},
		{"audio", MediaStream{Kind: StreamAudio}, "", ""},
	}
	for _, tt := range tests {
		if r, rt := tt.s.VideoRange(), tt.s.VideoRangeType(); r != tt.rng || rt != tt.rtype {
			t.Errorf("%s: got = %s %s, want = %s %s", tt.name, r, rt, tt.rng, tt.rtype)
		}
	}
}

func TestSubtitleCodecs(t *testing.T) {
	for codec, want := range map[string][3]bool{ // text, PGS, VobSub
		"subrip": {true, false, false}, "ass": {true, false, false}, "mov_text": {true, false, false},
		"PGSSUB": {false, true, false}, "sup": {false, true, false}, "DVDSUB": {false, false, true},
		"DVBSUB": {false, false, false}, "sub": {false, false, false}, "microdvd": {true, false, false},
	} {
		s := MediaStream{Kind: StreamSubtitle, Codec: codec}
		if got := [3]bool{s.IsTextSubtitle(), s.IsPGSSubtitle(), s.IsVobSubSubtitle()}; got != want {
			t.Errorf("%s: got = %v, want = %v", codec, got, want)
		}
	}
	if (&MediaStream{Kind: StreamAudio, Profile: "Dolby TrueHD + Dolby Atmos"}).SpatialFormat() != SpatialDolbyAtmos {
		t.Error("atmos: got = none")
	}
}

func TestCuratedItems(t *testing.T) {
	user := User{ID: NewID(), Policy: UserPolicy{Libraries: []ID{}}}
	lib := NewID()
	playlist := Item{ID: NewID(), LibraryID: lib, Kind: KindPlaylist, Name: "Road trip", UserID: user.ID}
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"collections library", (&Library{Name: "Collections", Kind: LibraryCollections}).Validate(), nil},
		{"collections library with paths", (&Library{Name: "Collections", Kind: LibraryCollections, Paths: []string{"/x"}}).Validate(), ErrInvalid},
		{"films without paths", (&Library{Name: "Films", Kind: LibraryMovies}).Validate(), ErrInvalid},
		{"playlist", playlist.Validate(), nil},
		{"playlist without user", (&Item{ID: NewID(), LibraryID: lib, Kind: KindPlaylist, Name: "x"}).Validate(), ErrInvalid},
		{"movie with user", (&Item{ID: NewID(), LibraryID: lib, Kind: KindMovie, Name: "x", UserID: user.ID}).Validate(), ErrInvalid},
		{"members and children", (&ItemQuery{MemberOf: NewID(), ParentID: NewID()}).Validate(), ErrInvalid},
		{"list order without list", (&ItemQuery{Sort: []SortSpec{{Field: SortListOrder}}}).Validate(), ErrInvalid},
		{"list order", (&ItemQuery{MemberOf: NewID(), Sort: []SortSpec{{Field: SortListOrder}}}).Validate(), nil},
	}
	for _, tt := range tests {
		if !errors.Is(tt.err, tt.want) || (tt.want == nil && tt.err != nil) {
			t.Errorf("%s: got = %v, want = %v", tt.name, tt.err, tt.want)
		}
	}

	// A user's own playlists are theirs whatever their library policy.
	if !user.CanAccess(&playlist) {
		t.Error("CanAccess(own playlist) = false")
	}
	other := playlist
	other.UserID = NewID()
	if user.CanAccess(&other) {
		t.Error("CanAccess(other's playlist) = true")
	}
	if !KindCollection.IsCurated() || !KindPlaylist.IsCurated() || KindFolder.IsCurated() {
		t.Error("IsCurated")
	}
}

func TestDisplayPreferences(t *testing.T) {
	user := NewID()
	tests := []struct {
		name string
		p    DisplayPreferences
		ok   bool
	}{
		{"valid", DisplayPreferences{UserID: user, Client: "web", View: "home", Values: map[string]string{"sort": "name"}}, true},
		{"no values", DisplayPreferences{UserID: user, Client: "web", View: "home"}, true},
		{"no user", DisplayPreferences{Client: "web", View: "home"}, false},
		{"no client", DisplayPreferences{UserID: user, View: "home"}, false},
		{"no view", DisplayPreferences{UserID: user, Client: "web"}, false},
		{"empty name", DisplayPreferences{UserID: user, Client: "web", View: "home", Values: map[string]string{"": "x"}}, false},
		{"long value", DisplayPreferences{UserID: user, Client: "web", View: "home", Values: map[string]string{"x": strings.Repeat("x", MaxDisplayValueBytes+1)}}, false},
	}
	for _, tt := range tests {
		if err := tt.p.Validate(); (err == nil) != tt.ok || (err != nil && !errors.Is(err, ErrInvalid)) {
			t.Errorf("%s: Validate() = %v", tt.name, err)
		}
	}
}
