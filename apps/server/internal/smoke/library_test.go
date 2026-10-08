package smoke_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/imaging"
	"github.com/mavioai/mavio/libs/metadata"
	"github.com/mavioai/mavio/libs/naming"
	"github.com/mavioai/mavio/libs/store"
	"github.com/mavioai/mavio/libs/subtitle"
)

const episodeNFO = `<?xml version="1.0" encoding="UTF-8"?>
<episodedetails>
  <title>Pilot</title>
  <season>1</season>
  <episode>2</episode>
  <plot>The one where it starts.</plot>
  <aired>2020-01-05</aired>
  <uniqueid type="tvdb">8888</uniqueid>
  <actor><name>张国荣</name><role>Lead</role><order>0</order></actor>
  <lockedfields>Overview</lockedfields>
</episodedetails>`

const episodeSRT = "1\n00:00:01,000 --> 00:00:02,500\n{\\i1}Hello{\\i0}\n\n2\n00:00:03,000 --> 00:00:04,000\nWorld\n"

// TestScanEpisodeFiles runs P2 end to end on the files of one episode, as
// a scan will: names are parsed into series, season and episode, the NFO
// file fills in metadata, the result is stored and found again, the
// subtitle is converted for a web client and the poster is resized.
func TestScanEpisodeFiles(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	lib := core.Library{Name: "Shows", Kind: core.LibraryShows, Paths: []string{"/media/shows"}}
	if err := s.Libraries().Create(ctx, &lib); err != nil {
		t.Fatal(err)
	}

	// Naming: series, season and episode from the paths.
	names := naming.Default()
	const (
		seriesDir = "/media/shows/Farewell Show (2020)"
		seasonDir = seriesDir + "/Season 01"
		video     = seasonDir + "/Farewell Show S01E02 1080p.mkv"
	)
	series := names.ResolveSeries(seriesDir)
	season := naming.ParseSeasonPath(seasonDir, seriesDir, true, true)
	ep, ok := names.ResolveEpisode(video, false, naming.EpisodeOptions{})
	if series.Name != "Farewell Show" || series.Year == nil || *series.Year != 2020 || !season.Success ||
		!ok || *ep.SeasonNumber != 1 || *ep.EpisodeNumber != 2 {
		t.Fatalf("naming: series %+v, season %+v, episode %+v", series, season, ep)
	}

	// Metadata: the episode's NFO file.
	md, err := metadata.ParseNFO([]byte(episodeNFO), core.KindEpisode, metadata.NFOOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Store the hierarchy.
	now := time.Now()
	seriesItem := core.Item{
		ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindSeries, Name: series.Name,
		ProductionYear: *series.Year, Path: seriesDir, DateAdded: now,
	}
	seasonItem := core.Item{
		ID: core.NewID(), LibraryID: lib.ID, ParentID: seriesItem.ID, Kind: core.KindSeason,
		Name: "Season 1", IndexNumber: season.SeasonNumber, Path: seasonDir, DateAdded: now,
	}
	episode := md.Item
	episode.ID, episode.LibraryID, episode.ParentID, episode.Path, episode.DateAdded = core.NewID(), lib.ID, seasonItem.ID, video, now
	if err := s.Items().Upsert(ctx, seriesItem, seasonItem, episode); err != nil {
		t.Fatal(err)
	}
	var credits []core.Credit
	for _, p := range md.People {
		person := core.Person{ID: core.NewID(), Name: p.Name}
		if err := s.People().Upsert(ctx, person); err != nil {
			t.Fatal(err)
		}
		credits = append(credits, core.Credit{PersonID: person.ID, Kind: p.Kind, Role: p.Role})
	}
	if err := s.People().ReplaceCredits(ctx, episode.ID, credits); err != nil {
		t.Fatal(err)
	}

	got, err := s.Items().Get(ctx, episode.ID)
	if err != nil || got.Name != "Pilot" || got.ExternalIDs[core.ProviderTVDB] != "8888" ||
		len(got.LockedFields) != 1 || got.LockedFields[0] != core.FieldOverview || got.PremiereDate == nil {
		t.Errorf("stored episode = %+v, %v", got, err)
	}
	page, err := s.Items().Query(ctx, core.ItemQuery{ParentID: seriesItem.ID, Recursive: true, Kinds: []core.ItemKind{core.KindEpisode}})
	if err != nil || page.Total != 1 {
		t.Errorf("episodes of the series = %d, %v", page.Total, err)
	}
	if people, err := s.People().Search(ctx, core.PersonQuery{Search: "zhang guo"}); err != nil || len(people) != 1 {
		t.Errorf("person by pinyin = %v, %v", people, err)
	}

	// Subtitle: the external SRT, recognized by name, served as WebVTT.
	ext, ok := names.ParseExternalFile(naming.ExternalSubtitle, nil, seasonDir+"/Farewell Show S01E02 1080p.forced.srt", ".forced")
	if !ok || !ext.IsForced {
		t.Errorf("external subtitle = %+v %v", ext, ok)
	}
	utf8, _, err := subtitle.ToUTF8([]byte(episodeSRT))
	if err != nil {
		t.Fatal(err)
	}
	vtt, err := subtitle.Convert(utf8, "srt", "vtt", 0, 0, false)
	if err != nil || !strings.HasPrefix(string(vtt), "WEBVTT") || !strings.Contains(string(vtt), "00:00:01.000 --> 00:00:02.500\nHello\n") {
		t.Errorf("subtitle = %q, %v", vtt, err)
	}

	// Imaging: a 600×900 poster requested at most 300 wide.
	var poster bytes.Buffer
	src := image.NewRGBA(image.Rect(0, 0, 600, 900))
	for i := range src.Pix {
		src.Pix[i] = uint8(i)
	}
	src.Set(0, 0, color.White)
	if err := png.Encode(&poster, src); err != nil {
		t.Fatal(err)
	}
	decoded, _, err := image.Decode(&poster)
	if err != nil {
		t.Fatal(err)
	}
	size := imaging.NewSize(imaging.SizeOptions{MaxWidth: 300}, imaging.Size{Width: 600, Height: 900})
	thumb := imaging.ResizeImage(decoded, size.Width, size.Height)
	if b := thumb.Bounds(); b.Dx() != 300 || b.Dy() != 450 {
		t.Errorf("poster = %v", b)
	}
}
