package main

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// External ID keys, as the host names providers.
const (
	keyArtist       = "musicbrainz_artist"
	keyAlbumArtist  = "musicbrainz_album_artist"
	keyAlbum        = "musicbrainz_album"
	keyReleaseGroup = "musicbrainz_release_group"
	keyTrack        = "musicbrainz_track"
)

// defaultLimit is the number of search results without a limit.
const defaultLimit = 10

func (p *plugin) Search(ctx context.Context, req *pluginv1.SearchRequest) (*pluginv1.SearchResponse, error) {
	c, l := p.client(), req.GetLookup()
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = defaultLimit
	}
	ids := l.GetExternalIds()
	var results []*pluginv1.SearchResult
	switch l.GetKind() {
	case pluginv1.MediaKind_MEDIA_KIND_MUSIC_ARTIST:
		if id := ids[keyArtist]; id != "" {
			a, err := c.artist(ctx, id)
			if err := ignoreNotFound(err); err != nil {
				return nil, err
			}
			if a.ID != "" {
				results = append(results, artistResult(a))
			}
			break
		}
		if l.GetName() == "" {
			break
		}
		artists, err := c.searchArtists(ctx, l.GetName(), limit)
		if err != nil {
			return nil, err
		}
		for _, a := range artists {
			results = append(results, artistResult(a))
		}
	case pluginv1.MediaKind_MEDIA_KIND_MUSIC_ALBUM:
		releases, err := p.findReleases(ctx, c, l, limit)
		if err != nil {
			return nil, err
		}
		for _, r := range releases {
			results = append(results, releaseResult(c, r))
		}
	case pluginv1.MediaKind_MEDIA_KIND_TRACK:
		if id := ids[keyTrack]; id != "" {
			r, err := c.recording(ctx, id)
			if err := ignoreNotFound(err); err != nil {
				return nil, err
			}
			if r.ID != "" {
				results = append(results, recordingResult(r))
			}
			break
		}
		if l.GetName() == "" {
			break
		}
		recordings, err := c.searchRecordings(ctx, l.GetName(), first(l.GetArtists()), l.GetAlbum(), limit)
		if err != nil {
			return nil, err
		}
		for _, r := range recordings {
			results = append(results, recordingResult(r))
		}
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return pluginv1.SearchResponse_builder{Results: results}.Build(), nil
}

// findReleases finds an album's releases: by its release or release group
// ID, else by its name and artist.
func (p *plugin) findReleases(ctx context.Context, c *client, l *pluginv1.Lookup, limit int) ([]mbRelease, error) {
	ids := l.GetExternalIds()
	if id := ids[keyAlbum]; id != "" {
		r, err := c.release(ctx, id)
		if err == nil {
			return []mbRelease{r}, nil
		}
		if !errors.Is(err, errNotFound) {
			return nil, err
		}
	}
	if id := ids[keyReleaseGroup]; id != "" {
		g, err := c.releaseGroup(ctx, id)
		if err == nil {
			for i := range g.Releases {
				g.Releases[i].ReleaseGroup = &mbReleaseGroup{ID: g.ID, Title: g.Title, FirstReleaseDate: g.FirstReleaseDate}
				if len(g.Releases[i].ArtistCredit) == 0 {
					g.Releases[i].ArtistCredit = g.ArtistCredit
				}
			}
			return g.Releases, nil
		}
		if !errors.Is(err, errNotFound) {
			return nil, err
		}
	}
	if l.GetName() == "" {
		return nil, nil
	}
	return c.searchReleases(ctx, l.GetName(), cmp.Or(ids[keyAlbumArtist], ids[keyArtist]), first(l.GetArtists()), limit)
}

func (p *plugin) GetMetadata(ctx context.Context, req *pluginv1.GetMetadataRequest) (*pluginv1.GetMetadataResponse, error) {
	c, l := p.client(), req.GetLookup()
	var md *pluginv1.Metadata
	var err error
	switch l.GetKind() {
	case pluginv1.MediaKind_MEDIA_KIND_MUSIC_ARTIST:
		md, err = p.artistMetadata(ctx, c, l)
	case pluginv1.MediaKind_MEDIA_KIND_MUSIC_ALBUM:
		md, err = p.albumMetadata(ctx, c, l)
	case pluginv1.MediaKind_MEDIA_KIND_TRACK:
		md, err = p.trackMetadata(ctx, c, l)
	}
	if err = ignoreNotFound(err); err != nil {
		return nil, err
	}
	if md == nil {
		return pluginv1.GetMetadataResponse_builder{Found: proto.Bool(false)}.Build(), nil
	}
	return pluginv1.GetMetadataResponse_builder{Found: proto.Bool(true), Metadata: md}.Build(), nil
}

func (p *plugin) artistMetadata(ctx context.Context, c *client, l *pluginv1.Lookup) (*pluginv1.Metadata, error) {
	id := l.GetExternalIds()[keyArtist]
	if id == "" {
		if l.GetName() == "" {
			return nil, nil
		}
		found, err := c.searchArtists(ctx, l.GetName(), 1)
		if err != nil || len(found) == 0 {
			return nil, err
		}
		id = found[0].ID
	}
	a, err := c.artist(ctx, id)
	if err != nil {
		return nil, err
	}
	md := pluginv1.Metadata_builder{
		Name:        proto.String(a.Name),
		SortName:    proto.String(a.SortName),
		Genres:      ranked(a.Genres),
		Tags:        ranked(a.Tags),
		ExternalIds: map[string]string{keyArtist: a.ID},
	}.Build()
	if d, year, ok := date(a.LifeSpan.Begin); ok {
		md.SetPremiereDate(d)
		md.SetProductionYear(year)
	}
	if d, _, ok := date(a.LifeSpan.End); ok {
		md.SetEndDate(d)
	}
	return md, nil
}

func (p *plugin) albumMetadata(ctx context.Context, c *client, l *pluginv1.Lookup) (*pluginv1.Metadata, error) {
	releases, err := p.findReleases(ctx, c, l, 1)
	if err != nil || len(releases) == 0 {
		return nil, err
	}
	// Search results lack what a lookup includes.
	r, err := c.release(ctx, releases[0].ID)
	if err != nil {
		return nil, err
	}
	var g mbReleaseGroup
	if r.ReleaseGroup != nil && r.ReleaseGroup.ID != "" {
		if g, err = c.releaseGroup(ctx, r.ReleaseGroup.ID); err != nil && !errors.Is(err, errNotFound) {
			return nil, err
		}
	}
	md := pluginv1.Metadata_builder{
		Name:         proto.String(r.Title),
		AlbumArtists: creditNames(either(r.ArtistCredit, g.ArtistCredit)),
		Genres:       ranked(either(g.Genres, r.Genres)),
		Tags:         ranked(either(g.Tags, r.Tags)),
		ExternalIds:  map[string]string{keyAlbum: r.ID},
	}.Build()
	md.SetArtists(md.GetAlbumArtists())
	for _, li := range r.LabelInfo {
		if li.Label != nil && li.Label.Name != "" && !slices.Contains(md.GetStudios(), li.Label.Name) {
			md.SetStudios(append(md.GetStudios(), li.Label.Name))
		}
	}
	if g.ID != "" {
		md.GetExternalIds()[keyReleaseGroup] = g.ID
	}
	if credits := either(r.ArtistCredit, g.ArtistCredit); len(credits) > 0 && credits[0].Artist.ID != "" {
		md.GetExternalIds()[keyAlbumArtist] = credits[0].Artist.ID
	}
	// The release group's first release dates the album.
	if d, year, ok := date(cmp.Or(g.FirstReleaseDate, r.Date)); ok {
		md.SetPremiereDate(d)
		md.SetProductionYear(year)
	}
	cover := c.coverURL("release-group", g.ID)
	if r.CoverArt.Front || g.ID == "" {
		cover = c.coverURL("release", r.ID)
	}
	md.SetImages([]*pluginv1.RemoteImage{pluginv1.RemoteImage_builder{
		Kind: pluginv1.ImageKind_IMAGE_KIND_PRIMARY.Enum(), Url: proto.String(cover),
	}.Build()})
	return md, nil
}

func (p *plugin) trackMetadata(ctx context.Context, c *client, l *pluginv1.Lookup) (*pluginv1.Metadata, error) {
	id := l.GetExternalIds()[keyTrack]
	if id == "" {
		if l.GetName() == "" {
			return nil, nil
		}
		found, err := c.searchRecordings(ctx, l.GetName(), first(l.GetArtists()), l.GetAlbum(), 1)
		if err != nil || len(found) == 0 {
			return nil, err
		}
		id = found[0].ID
	}
	r, err := c.recording(ctx, id)
	if err != nil {
		return nil, err
	}
	md := pluginv1.Metadata_builder{
		Name:        proto.String(r.Title),
		Artists:     creditNames(r.ArtistCredit),
		Genres:      ranked(r.Genres),
		Tags:        ranked(r.Tags),
		ExternalIds: map[string]string{keyTrack: r.ID},
	}.Build()
	if r.Length > 0 {
		md.SetRuntime(durationpb.New(time.Duration(r.Length) * time.Millisecond))
	}
	if d, year, ok := date(r.FirstReleaseDate); ok {
		md.SetPremiereDate(d)
		md.SetProductionYear(year)
	}
	if len(r.ArtistCredit) > 0 && r.ArtistCredit[0].Artist.ID != "" {
		md.GetExternalIds()[keyArtist] = r.ArtistCredit[0].Artist.ID
	}
	return md, nil
}

func artistResult(a mbArtist) *pluginv1.SearchResult {
	name := a.Name
	if a.Disambiguation != "" {
		name += " (" + a.Disambiguation + ")"
	}
	r := pluginv1.SearchResult_builder{
		Name:        proto.String(name),
		ExternalIds: map[string]string{keyArtist: a.ID},
		Score:       proto.Float64(score(a.Score)),
	}.Build()
	if _, year, ok := date(a.LifeSpan.Begin); ok {
		r.SetYear(year)
	}
	return r
}

func releaseResult(c *client, rel mbRelease) *pluginv1.SearchResult {
	ids := map[string]string{keyAlbum: rel.ID}
	dated := rel.Date
	if rel.ReleaseGroup != nil && rel.ReleaseGroup.ID != "" {
		ids[keyReleaseGroup] = rel.ReleaseGroup.ID
		dated = cmp.Or(rel.ReleaseGroup.FirstReleaseDate, dated)
	}
	if len(rel.ArtistCredit) > 0 && rel.ArtistCredit[0].Artist.ID != "" {
		ids[keyAlbumArtist] = rel.ArtistCredit[0].Artist.ID
	}
	name := rel.Title
	if artists := creditNames(rel.ArtistCredit); len(artists) > 0 {
		name = strings.Join(artists, ", ") + " – " + rel.Title
	}
	r := pluginv1.SearchResult_builder{
		Name:        proto.String(name),
		Overview:    proto.String(strings.TrimSpace(rel.Country + " " + rel.Date)),
		ExternalIds: ids,
		ImageUrl:    proto.String(c.coverURL("release", rel.ID)),
		Score:       proto.Float64(score(rel.Score)),
	}.Build()
	if _, year, ok := date(dated); ok {
		r.SetYear(year)
	}
	return r
}

func recordingResult(rec mbRecording) *pluginv1.SearchResult {
	name := rec.Title
	if artists := creditNames(rec.ArtistCredit); len(artists) > 0 {
		name = strings.Join(artists, ", ") + " – " + rec.Title
	}
	r := pluginv1.SearchResult_builder{
		Name:        proto.String(name),
		ExternalIds: map[string]string{keyTrack: rec.ID},
		Score:       proto.Float64(score(rec.Score)),
	}.Build()
	if len(rec.Releases) > 0 {
		r.SetOverview(rec.Releases[0].Title)
	}
	if _, year, ok := date(rec.FirstReleaseDate); ok {
		r.SetYear(year)
	}
	return r
}

// score turns a search score of 0–100 into one of 0–1; lookups, which
// have none, are certain.
func score(s int) float64 {
	if s == 0 {
		return 1
	}
	return float64(s) / 100
}

// either returns a unless it is empty, else b.
func either[T any](a, b []T) []T {
	if len(a) > 0 {
		return a
	}
	return b
}

// creditNames names the artists of a credit.
func creditNames(credits []mbArtistCredit) []string {
	var out []string
	for _, c := range credits {
		if name := cmp.Or(c.Artist.Name, c.Name); name != "" && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out
}

// ranked lists genres or tags, the most voted first.
func ranked(counts []mbCount) []string {
	counts = slices.Clone(counts)
	slices.SortStableFunc(counts, func(a, b mbCount) int { return cmp.Compare(b.Count, a.Count) })
	var out []string
	for _, c := range counts {
		if c.Name != "" {
			out = append(out, c.Name)
		}
	}
	return out
}

// date parses a MusicBrainz date: a year, a month or a day. It returns
// the ISO 8601 date, its first day where the day or month is unknown, and
// the year.
func date(s string) (string, int32, bool) {
	parts := strings.Split(s, "-")
	if len(parts) == 0 || len(parts) > 3 || len(parts[0]) != 4 {
		return "", 0, false
	}
	year, err := strconv.Atoi(parts[0])
	if err != nil || year <= 0 {
		return "", 0, false
	}
	for len(parts) < 3 {
		parts = append(parts, "01")
	}
	d := strings.Join(parts, "-")
	if _, err := time.Parse(time.DateOnly, d); err != nil {
		return "", 0, false
	}
	return d, int32(year), true
}

func first(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

// ignoreNotFound treats unknown IDs as nothing found.
func ignoreNotFound(err error) error {
	if errors.Is(err, errNotFound) {
		return nil
	}
	return err
}
