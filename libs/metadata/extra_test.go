package metadata

import (
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

func TestParseNFOLocksAndURLs(t *testing.T) {
	nfo := `<?xml version="1.0"?>
<tvshow>
  <title>Show</title>
  <lockdata>true</lockdata>
  <lockedfields>Cast|Genres|OfficialRating|Bogus</lockedfields>
  <displayorder>dvd</displayorder>
  <airs_dayofweek>Daily</airs_dayofweek>
  <status>Returning Series</status>
  <namedseason number="1">Pilot Season</namedseason>
  <writer>Ann | Bob; Cy, Jr.</writer>
  <actor><name>Ann</name><role>Herself</role></actor>
  <actor><name>Ann</name><type>GuestStar</type></actor>
</tvshow>
https://www.thetvdb.com/?tab=series&id=121361`
	r, err := ParseNFO([]byte(nfo), core.KindSeries, NFOOptions{})
	if err != nil {
		t.Fatal(err)
	}
	it := r.Item
	if !it.Locked || !slices.Equal(it.LockedFields, []core.MetadataField{core.FieldCast, core.FieldGenres, core.FieldOfficialRating}) {
		t.Errorf("locks: got = %v %q", it.Locked, it.LockedFields)
	}
	check(t, "display order", it.DisplayOrder, "dvd")
	check(t, "air days", len(it.AirDays), 7)
	check(t, "status", it.SeriesStatus, core.SeriesContinuing)
	check(t, "tvdb", it.ExternalIDs[core.ProviderTVDB], "121361")
	if w := names(peopleOf(r, core.CreditWriter)); !slices.Equal(w, []string{"Ann", "Bob", "Cy, Jr."}) {
		t.Errorf("writers: got = %q", w)
	}
	// The guest star credit promotes the actor credit instead of
	// duplicating it.
	cast := peopleOf(r, core.CreditGuestStar)
	if len(cast) != 1 || cast[0].Role != "Herself" || len(peopleOf(r, core.CreditActor)) != 0 {
		t.Errorf("cast: got = %+v", r.People)
	}

	if name, ok := SeasonNameFromSeriesNFO([]byte(nfo), 1); !ok || name != "Pilot Season" {
		t.Errorf("season name: got = %q %v", name, ok)
	}
	if _, ok := SeasonNameFromSeriesNFO([]byte(nfo), 2); ok {
		t.Error("season 2: got = a name, want = none")
	}
}

func TestParseNFOMalformed(t *testing.T) {
	// Unclosed elements and HTML entities do not lose what came before.
	r, err := ParseNFO([]byte("<movie><title>Caf&eacute;</title><plot>Broken <b>markup"), core.KindMovie, NFOOptions{})
	if err != nil {
		t.Fatal(err)
	}
	check(t, "name", r.Item.Name, "Café")
	if _, err := ParseNFO([]byte("not xml at all"), core.KindSeason, NFOOptions{}); err == nil {
		t.Error("garbage: got = no error, want = error")
	}
}
