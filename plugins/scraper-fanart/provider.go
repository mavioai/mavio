package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// External ID keys, as the host names providers.
const (
	keyTMDB          = "tmdb"
	keyIMDb          = "imdb"
	keyTVDB          = "tvdb"
	keyMBArtist      = "musicbrainz_artist"
	keyMBGroup       = "musicbrainz_release_group"
	keyMBAlbumArtist = "musicbrainz_album_artist"
)

// Search finds nothing: fanart.tv only has images of what other
// providers identified.
func (p *plugin) Search(context.Context, *pluginv1.SearchRequest) (*pluginv1.SearchResponse, error) {
	return &pluginv1.SearchResponse{}, nil
}

func (p *plugin) GetMetadata(ctx context.Context, req *pluginv1.GetMetadataRequest) (*pluginv1.GetMetadataResponse, error) {
	c, err := p.client()
	if err != nil {
		return nil, err
	}
	l := req.GetLookup()
	ids, lang := l.GetExternalIds(), l.GetLanguage()
	var images []*pluginv1.RemoteImage
	switch l.GetKind() {
	case pluginv1.MediaKind_MEDIA_KIND_MOVIE:
		if id := cmp.Or(ids[keyTMDB], ids[keyIMDb]); id != "" {
			images, err = p.entity(ctx, c, "movies/"+url.PathEscape(id), movieKinds, lang, nil)
		}
	case pluginv1.MediaKind_MEDIA_KIND_SERIES:
		if id := ids[keyTVDB]; id != "" {
			images, err = p.entity(ctx, c, "tv/"+url.PathEscape(id), seriesKinds, lang, nil)
		}
	case pluginv1.MediaKind_MEDIA_KIND_SEASON:
		if id := l.GetSeriesExternalIds()[keyTVDB]; id != "" && l.HasSeasonNumber() {
			season := strconv.Itoa(int(l.GetSeasonNumber()))
			images, err = p.entity(ctx, c, "tv/"+url.PathEscape(id), seasonKinds, lang, func(img fanartImage) bool {
				return img.Season == season
			})
		}
	case pluginv1.MediaKind_MEDIA_KIND_MUSIC_ARTIST:
		if id := cmp.Or(ids[keyMBArtist], ids[keyMBAlbumArtist]); id != "" {
			images, err = p.entity(ctx, c, "music/"+url.PathEscape(id), artistKinds, lang, nil)
		}
	case pluginv1.MediaKind_MEDIA_KIND_MUSIC_ALBUM:
		if id := ids[keyMBGroup]; id != "" {
			images, err = p.album(ctx, c, id, lang)
		}
	}
	if errors.Is(err, errNotFound) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	if len(images) == 0 {
		return pluginv1.GetMetadataResponse_builder{Found: proto.Bool(false)}.Build(), nil
	}
	md := pluginv1.Metadata_builder{Images: images}.Build()
	return pluginv1.GetMetadataResponse_builder{Found: proto.Bool(true), Metadata: md}.Build(), nil
}

func (p *plugin) entity(ctx context.Context, c *client, path string, kinds []field, lang string, keep func(fanartImage) bool) ([]*pluginv1.RemoteImage, error) {
	fields, err := c.images(ctx, path)
	if err != nil {
		return nil, err
	}
	return collect(fields, kinds, lang, keep), nil
}

// album finds a release group's images, listed by release group under
// "albums".
func (p *plugin) album(ctx context.Context, c *client, id, lang string) ([]*pluginv1.RemoteImage, error) {
	fields, err := c.images(ctx, "music/albums/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	var albums map[string]map[string]json.RawMessage
	if err := json.Unmarshal(fields["albums"], &albums); err != nil {
		return nil, nil
	}
	return collect(albums[id], albumKinds, lang, nil), nil
}
