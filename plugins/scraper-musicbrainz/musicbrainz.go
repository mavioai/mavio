package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// userAgent identifies the plugin, as MusicBrainz requires.
const userAgent = "Mavio/0.1 ( https://github.com/mavioai/mavio )"

// errNotFound is MusicBrainz's answer for unknown IDs.
var errNotFound = errors.New("musicbrainz: not found")

// client calls the MusicBrainz web service (ws/2) and the Cover Art
// Archive.
type client struct {
	http        *http.Client
	baseURL     string
	coverArtURL string
	limiter     *limiter
}

func (c *client) get(ctx context.Context, path string, query url.Values, out any) error {
	if err := c.limiter.wait(ctx); err != nil {
		return err
	}
	query.Set("fmt", "json")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/ws/2/"+path+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("musicbrainz %s: %w", path, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return errNotFound
	case resp.StatusCode != http.StatusOK:
		var body struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&body)
		return fmt.Errorf("musicbrainz %s: %s: %s", path, resp.Status, body.Error)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("musicbrainz %s: %w", path, err)
	}
	return nil
}

// lookup fetches an entity by MBID with the given includes.
func (c *client) lookup(ctx context.Context, entity, id, inc string, out any) error {
	q := url.Values{}
	if inc != "" {
		q.Set("inc", inc)
	}
	return c.get(ctx, entity+"/"+url.PathEscape(id), q, out)
}

// search runs a Lucene query against an entity's index.
func (c *client) search(ctx context.Context, entity, query string, limit int, out any) error {
	return c.get(ctx, entity, url.Values{"query": {query}, "limit": {strconv.Itoa(limit)}}, out)
}

// luceneSpecial are the characters Lucene queries escape.
const luceneSpecial = `+-&|!(){}[]^"~*?:\/`

// phrase quotes a value as a Lucene phrase.
func phrase(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		if strings.ContainsRune(luceneSpecial, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

type mbArtistCredit struct {
	Name       string `json:"name"`
	JoinPhrase string `json:"joinphrase"`
	Artist     struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		SortName string `json:"sort-name"`
	} `json:"artist"`
}

type mbCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type mbLifeSpan struct {
	Begin string `json:"begin"`
	End   string `json:"end"`
	Ended bool   `json:"ended"`
}

type mbArtist struct {
	ID             string     `json:"id"`
	Score          int        `json:"score"`
	Name           string     `json:"name"`
	SortName       string     `json:"sort-name"`
	Type           string     `json:"type"`
	Country        string     `json:"country"`
	Disambiguation string     `json:"disambiguation"`
	LifeSpan       mbLifeSpan `json:"life-span"`
	Genres         []mbCount  `json:"genres"`
	Tags           []mbCount  `json:"tags"`
	Area           *struct {
		Name string `json:"name"`
	} `json:"area"`
}

type mbReleaseGroup struct {
	ID               string           `json:"id"`
	Title            string           `json:"title"`
	PrimaryType      string           `json:"primary-type"`
	FirstReleaseDate string           `json:"first-release-date"`
	ArtistCredit     []mbArtistCredit `json:"artist-credit"`
	Genres           []mbCount        `json:"genres"`
	Tags             []mbCount        `json:"tags"`
	Releases         []mbRelease      `json:"releases"`
}

type mbRelease struct {
	ID           string           `json:"id"`
	Score        int              `json:"score"`
	Title        string           `json:"title"`
	Date         string           `json:"date"`
	Country      string           `json:"country"`
	ArtistCredit []mbArtistCredit `json:"artist-credit"`
	ReleaseGroup *mbReleaseGroup  `json:"release-group"`
	Genres       []mbCount        `json:"genres"`
	Tags         []mbCount        `json:"tags"`
	LabelInfo    []struct {
		Label *struct {
			Name string `json:"name"`
		} `json:"label"`
	} `json:"label-info"`
	CoverArt struct {
		Front bool `json:"front"`
	} `json:"cover-art-archive"`
}

type mbRecording struct {
	ID               string           `json:"id"`
	Score            int              `json:"score"`
	Title            string           `json:"title"`
	Length           int64            `json:"length"` // milliseconds
	FirstReleaseDate string           `json:"first-release-date"`
	ArtistCredit     []mbArtistCredit `json:"artist-credit"`
	Releases         []mbRelease      `json:"releases"`
	Genres           []mbCount        `json:"genres"`
	Tags             []mbCount        `json:"tags"`
}

func (c *client) artist(ctx context.Context, id string) (mbArtist, error) {
	var a mbArtist
	return a, c.lookup(ctx, "artist", id, "genres+tags", &a)
}

func (c *client) searchArtists(ctx context.Context, name string, limit int) ([]mbArtist, error) {
	var out struct {
		Artists []mbArtist `json:"artists"`
	}
	return out.Artists, c.search(ctx, "artist", "artist:"+phrase(name), limit, &out)
}

func (c *client) release(ctx context.Context, id string) (mbRelease, error) {
	var r mbRelease
	return r, c.lookup(ctx, "release", id, "artists+release-groups+labels+genres+tags", &r)
}

func (c *client) releaseGroup(ctx context.Context, id string) (mbReleaseGroup, error) {
	var g mbReleaseGroup
	return g, c.lookup(ctx, "release-group", id, "artists+genres+tags+releases", &g)
}

// searchReleases finds releases by title, by an artist given by MBID or
// name when known.
func (c *client) searchReleases(ctx context.Context, title, artistID, artist string, limit int) ([]mbRelease, error) {
	q := "release:" + phrase(title)
	switch {
	case artistID != "":
		q += " AND arid:" + phrase(artistID)
	case artist != "":
		q += " AND artist:" + phrase(artist)
	}
	var out struct {
		Releases []mbRelease `json:"releases"`
	}
	return out.Releases, c.search(ctx, "release", q, limit, &out)
}

func (c *client) recording(ctx context.Context, id string) (mbRecording, error) {
	var r mbRecording
	return r, c.lookup(ctx, "recording", id, "artists+genres+tags+releases", &r)
}

// searchRecordings finds recordings by title, by an artist and on an
// album when known.
func (c *client) searchRecordings(ctx context.Context, title, artist, album string, limit int) ([]mbRecording, error) {
	q := "recording:" + phrase(title)
	if artist != "" {
		q += " AND artist:" + phrase(artist)
	}
	if album != "" {
		q += " AND release:" + phrase(album)
	}
	var out struct {
		Recordings []mbRecording `json:"recordings"`
	}
	return out.Recordings, c.search(ctx, "recording", q, limit, &out)
}

// coverURL is the Cover Art Archive's front cover of a release, or of a
// release group.
func (c *client) coverURL(entity, id string) string {
	return c.coverArtURL + "/" + entity + "/" + url.PathEscape(id) + "/front"
}
