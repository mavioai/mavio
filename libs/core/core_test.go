package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
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
		{"library without paths", (&Library{Name: "x", Kind: LibraryMovies}).Validate()},
		{"library unknown kind", (&Library{Name: "x", Kind: "tv", Paths: []string{"/"}}).Validate()},
		{"user without credentials", (&User{ID: NewID(), Name: "bob"}).Validate()},
		{"user bad subtitle mode", (&User{ID: NewID(), Name: "bob", AuthProvider: "ldap", Preferences: UserPreferences{SubtitleMode: "sometimes"}}).Validate()},
		{"user data rating", (&UserData{UserID: NewID(), ItemID: NewID(), Rating: &ten}).Validate()},
		{"user data position", (&UserData{UserID: NewID(), ItemID: NewID(), Position: -time.Second}).Validate()},
		{"job without attempts", (&Job{ID: NewID(), Kind: "x"}).Validate()},
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
