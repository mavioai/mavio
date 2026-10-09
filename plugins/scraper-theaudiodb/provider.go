package main

import (
	"cmp"
	"context"
	"net/url"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// External ID keys, as the host names providers.
const (
	keyArtist        = "audiodb_artist"
	keyAlbum         = "audiodb_album"
	keyMBArtist      = "musicbrainz_artist"
	keyMBAlbumArtist = "musicbrainz_album_artist"
	keyMBGroup       = "musicbrainz_release_group"
)

// languageSuffixes are TheAudioDB's suffixes of texts by ISO 639-1 code,
// as Jellyfin maps them.
var languageSuffixes = map[string]string{
	"de": "DE", "en": "EN", "es": "ES", "fr": "FR", "he": "IL", "hu": "HU", "it": "IT", "ja": "JP", "nl": "NL",
	"no": "NO", "nb": "NO", "nn": "NO", "pl": "PL", "pt": "PT", "ru": "RU", "sv": "SE", "zh": "CN",
}

// text picks the text in a language, else in English, else the one
// without a language.
func text(texts map[string]string, language string) string {
	lang, _, _ := strings.Cut(strings.ToLower(language), "-")
	return cmp.Or(texts[languageSuffixes[lang]], texts["EN"], texts[""])
}

func (p *plugin) findArtists(ctx context.Context, c *client, l *pluginv1.Lookup) ([]adbArtist, error) {
	ids := l.GetExternalIds()
	if id := ids[keyArtist]; id != "" {
		return c.artists(ctx, "artist.php", url.Values{"i": {id}})
	}
	if id := cmp.Or(ids[keyMBArtist], ids[keyMBAlbumArtist]); id != "" {
		found, err := c.artists(ctx, "artist-mb.php", url.Values{"i": {id}})
		if err != nil || len(found) > 0 {
			return found, err
		}
	}
	if l.GetName() == "" {
		return nil, nil
	}
	return c.artists(ctx, "search.php", url.Values{"s": {l.GetName()}})
}

func (p *plugin) findAlbums(ctx context.Context, c *client, l *pluginv1.Lookup) ([]adbAlbum, error) {
	ids := l.GetExternalIds()
	if id := ids[keyAlbum]; id != "" {
		return c.albums(ctx, "album.php", url.Values{"m": {id}})
	}
	if id := ids[keyMBGroup]; id != "" {
		found, err := c.albums(ctx, "album-mb.php", url.Values{"i": {id}})
		if err != nil || len(found) > 0 {
			return found, err
		}
	}
	if l.GetName() == "" || len(l.GetArtists()) == 0 {
		return nil, nil
	}
	return c.albums(ctx, "searchalbum.php", url.Values{"s": {l.GetArtists()[0]}, "a": {l.GetName()}})
}

func (p *plugin) Search(ctx context.Context, req *pluginv1.SearchRequest) (*pluginv1.SearchResponse, error) {
	c, err := p.client()
	if err != nil {
		return nil, err
	}
	l := req.GetLookup()
	var results []*pluginv1.SearchResult
	switch l.GetKind() {
	case pluginv1.MediaKind_MEDIA_KIND_MUSIC_ARTIST:
		artists, err := p.findArtists(ctx, c, l)
		if err != nil {
			return nil, err
		}
		for _, a := range artists {
			results = append(results, pluginv1.SearchResult_builder{
				Name: proto.String(a.Name), Year: proto.Int32(year(cmp.Or(a.FormedYear, a.BornYear))),
				Overview: proto.String(text(a.Biographies, l.GetLanguage())), ExternalIds: artistIDs(a),
				ImageUrl: proto.String(a.Thumb), Score: proto.Float64(1),
			}.Build())
		}
	case pluginv1.MediaKind_MEDIA_KIND_MUSIC_ALBUM:
		albums, err := p.findAlbums(ctx, c, l)
		if err != nil {
			return nil, err
		}
		for _, a := range albums {
			results = append(results, pluginv1.SearchResult_builder{
				Name: proto.String(strings.TrimPrefix(a.Artist+" – ", " – ") + a.Name), Year: proto.Int32(year(a.YearReleased)),
				Overview: proto.String(text(a.Descriptions, l.GetLanguage())), ExternalIds: albumIDs(a),
				ImageUrl: proto.String(a.Thumb), Score: proto.Float64(1),
			}.Build())
		}
	}
	if limit := int(req.GetLimit()); limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return pluginv1.SearchResponse_builder{Results: results}.Build(), nil
}

func (p *plugin) GetMetadata(ctx context.Context, req *pluginv1.GetMetadataRequest) (*pluginv1.GetMetadataResponse, error) {
	c, err := p.client()
	if err != nil {
		return nil, err
	}
	l := req.GetLookup()
	var md *pluginv1.Metadata
	switch l.GetKind() {
	case pluginv1.MediaKind_MEDIA_KIND_MUSIC_ARTIST:
		artists, err := p.findArtists(ctx, c, l)
		if err != nil {
			return nil, err
		}
		if len(artists) > 0 {
			md = artistMetadata(artists[0], l.GetLanguage())
		}
	case pluginv1.MediaKind_MEDIA_KIND_MUSIC_ALBUM:
		albums, err := p.findAlbums(ctx, c, l)
		if err != nil {
			return nil, err
		}
		if len(albums) > 0 {
			md = albumMetadata(albums[0], l.GetLanguage())
		}
	}
	if md == nil {
		return pluginv1.GetMetadataResponse_builder{Found: proto.Bool(false)}.Build(), nil
	}
	return pluginv1.GetMetadataResponse_builder{Found: proto.Bool(true), Metadata: md}.Build(), nil
}

func artistIDs(a adbArtist) map[string]string {
	ids := map[string]string{keyArtist: a.ID}
	if a.MusicBrainzID != "" {
		ids[keyMBArtist] = a.MusicBrainzID
	}
	return ids
}

func albumIDs(a adbAlbum) map[string]string {
	ids := map[string]string{keyAlbum: a.ID}
	if a.MusicBrainzID != "" {
		ids[keyMBGroup] = a.MusicBrainzID
	}
	if a.MusicBrainzArtistID != "" {
		ids[keyMBAlbumArtist] = a.MusicBrainzArtistID
	}
	if a.ArtistID != "" {
		ids[keyArtist] = a.ArtistID
	}
	return ids
}

func artistMetadata(a adbArtist, language string) *pluginv1.Metadata {
	md := pluginv1.Metadata_builder{
		Name:        proto.String(a.Name),
		Overview:    proto.String(text(a.Biographies, language)),
		Genres:      list(a.Genre, a.Style),
		Tags:        list(a.Mood),
		Studios:     list(a.Label),
		ExternalIds: artistIDs(a),
	}.Build()
	if y := year(cmp.Or(a.FormedYear, a.BornYear)); y > 0 {
		md.SetProductionYear(y)
		md.SetPremiereDate(strconv.Itoa(int(y)) + "-01-01")
	}
	if y := year(cmp.Or(a.DiedYear, a.Disbanded)); y > 0 {
		md.SetEndDate(strconv.Itoa(int(y)) + "-01-01")
	}
	md.SetImages(images(
		image{pluginv1.ImageKind_IMAGE_KIND_PRIMARY, a.Thumb},
		image{pluginv1.ImageKind_IMAGE_KIND_LOGO, a.Logo},
		image{pluginv1.ImageKind_IMAGE_KIND_BANNER, a.Banner},
		image{pluginv1.ImageKind_IMAGE_KIND_ART, a.Clearart},
		image{pluginv1.ImageKind_IMAGE_KIND_THUMB, a.WideThumb},
		image{pluginv1.ImageKind_IMAGE_KIND_BACKDROP, a.Fanart},
		image{pluginv1.ImageKind_IMAGE_KIND_BACKDROP, a.Fanart2},
		image{pluginv1.ImageKind_IMAGE_KIND_BACKDROP, a.Fanart3},
		image{pluginv1.ImageKind_IMAGE_KIND_BACKDROP, a.Fanart4},
	))
	return md
}

func albumMetadata(a adbAlbum, language string) *pluginv1.Metadata {
	md := pluginv1.Metadata_builder{
		Name:        proto.String(a.Name),
		Overview:    proto.String(text(a.Descriptions, language)),
		Genres:      list(a.Genre, a.Style),
		Tags:        list(a.Mood),
		Studios:     list(a.Label),
		ExternalIds: albumIDs(a),
	}.Build()
	if a.Artist != "" {
		md.SetAlbumArtists([]string{a.Artist})
	}
	if y := year(a.YearReleased); y > 0 {
		md.SetProductionYear(y)
	}
	if score, err := strconv.ParseFloat(a.Score, 64); err == nil && score > 0 && score <= 10 {
		md.SetCommunityRating(score)
	}
	md.SetImages(images(
		image{pluginv1.ImageKind_IMAGE_KIND_PRIMARY, a.Thumb},
		image{pluginv1.ImageKind_IMAGE_KIND_DISC, a.CDArt},
	))
	return md
}

type image struct {
	kind pluginv1.ImageKind
	url  string
}

func images(list ...image) []*pluginv1.RemoteImage {
	var out []*pluginv1.RemoteImage
	for _, img := range list {
		if img.url != "" {
			out = append(out, pluginv1.RemoteImage_builder{Kind: img.kind.Enum(), Url: proto.String(img.url)}.Build())
		}
	}
	return out
}

// list returns the non-empty values, split where several are joined with
// slashes.
func list(values ...string) []string {
	var out []string
	for _, v := range values {
		for part := range strings.SplitSeq(v, "/") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// year parses a year; zero when unknown.
func year(s string) int32 {
	y, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || y < 1000 || y > 9999 {
		return 0
	}
	return int32(y)
}
