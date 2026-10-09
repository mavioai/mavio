package metadata

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func TestWriteNFORoundTrip(t *testing.T) {
	date := func(s string) *time.Time {
		d, _ := time.Parse(time.DateOnly, s)
		return &d
	}
	num := func(n int) *int { return &n }
	tests := []struct {
		name string
		in   Result
	}{
		{"movie", Result{
			Item: core.Item{
				Kind: core.KindMovie, Name: "Farewell My Concubine", OriginalTitle: "霸王别姬", SortName: "Farewell",
				Overview: "Two boys meet <at> an opera school & more.", Tagline: "A tale", ProductionYear: 1993,
				PremiereDate: date("1993-01-01"), Runtime: 171 * time.Minute, OfficialRating: "R", CustomRating: "PG-13",
				CommunityRating: 8.1, CriticRating: 91, Genres: []string{"Drama", "Romance"}, Tags: []string{"opera"},
				Studios: []string{"Tomson"}, ProductionLocations: []string{"China", "Hong Kong"},
				ExternalIDs:    map[core.Provider]string{core.ProviderTMDB: "10997", core.ProviderIMDb: "tt0106332", core.ProviderTMDBCollection: "42"},
				CollectionName: "Chen Kaige", Locked: true, LockedFields: []core.MetadataField{core.FieldName, core.FieldOfficialRating, core.FieldProductionLocations},
			},
			People: []Person{
				{Name: "张国荣", Kind: core.CreditActor, Role: "Cheng Dieyi", Order: num(0)},
				{Name: "陈凯歌", Kind: core.CreditDirector},
			},
			RemoteImages: []RemoteImage{
				{Kind: core.ImagePrimary, URL: "https://example.com/poster.jpg"},
				{Kind: core.ImageLogo, URL: "https://example.com/logo.png"},
				{Kind: core.ImageBackdrop, URL: "https://example.com/fanart.jpg"},
			},
		}},
		{"series", Result{Item: core.Item{
			Kind: core.KindSeries, Name: "Show", SeriesStatus: core.SeriesEnded, AirDays: []time.Weekday{time.Monday},
			AirTime: "9 PM", DisplayOrder: "dvd", EndDate: date("2001-02-03"), ExternalIDs: map[core.Provider]string{core.ProviderTVDB: "7"},
		}}},
		{"season", Result{Item: core.Item{Kind: core.KindSeason, Name: "Season 2", IndexNumber: num(2)}}},
		{"episode", Result{
			Item: core.Item{
				Kind: core.KindEpisode, Name: "Pilot", ParentIndexNumber: num(1), IndexNumber: num(1), IndexNumberEnd: num(2),
				AirsBeforeSeasonNumber: num(2), AirsBeforeEpisodeNumber: num(3),
			},
			SeriesName: "Show",
		}},
		{"music video", Result{Item: core.Item{Kind: core.KindMusicVideo, Name: "Song", Artists: []string{"A", "B"}, Album: "LP"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := WriteNFO(&tt.in)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseNFO(data, tt.in.Item.Kind, NFOOptions{})
			if err != nil {
				t.Fatalf("ParseNFO: %v\n%s", err, data)
			}
			if !reflect.DeepEqual(got.Item, tt.in.Item) {
				t.Errorf("item:\ngot  = %+v\nwant = %+v\n%s", got.Item, tt.in.Item, data)
			}
			if !reflect.DeepEqual(got.People, tt.in.People) {
				t.Errorf("people: got = %+v, want = %+v", got.People, tt.in.People)
			}
			if !reflect.DeepEqual(got.RemoteImages, tt.in.RemoteImages) {
				t.Errorf("images: got = %+v, want = %+v", got.RemoteImages, tt.in.RemoteImages)
			}
			if got.SeriesName != tt.in.SeriesName {
				t.Errorf("series name: got = %q, want = %q", got.SeriesName, tt.in.SeriesName)
			}
		})
	}
}

func TestWriteNFOUnsupportedKind(t *testing.T) {
	if _, err := WriteNFO(&Result{Item: core.Item{Kind: core.KindPhoto}}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("WriteNFO(photo) error = %v, want ErrInvalid", err)
	}
	if CanWriteNFO(core.KindPhoto) || !CanWriteNFO(core.KindMovie) {
		t.Error("CanWriteNFO")
	}
}

func TestWriteNFOEscapes(t *testing.T) {
	data, err := WriteNFO(&Result{Item: core.Item{Kind: core.KindMovie, Name: "A & B </title>"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<title>A &amp; B &lt;/title&gt;</title>") {
		t.Errorf("got = %s", data)
	}
}
