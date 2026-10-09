package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// client calls TheAudioDB's API.
type client struct {
	http    *http.Client
	baseURL string
	key     string
}

// get fetches an endpoint; TheAudioDB answers unknown IDs with null lists.
func (c *client) get(ctx context.Context, endpoint string, query url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/"+url.PathEscape(c.key)+"/"+endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		// Do's error carries the URL, and with it the API key.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return fmt.Errorf("theaudiodb %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("theaudiodb %s: %s", endpoint, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("theaudiodb %s: %w", endpoint, err)
	}
	return nil
}

// adbArtist is an artist; empty strings and nulls mean unknown.
type adbArtist struct {
	ID            string            `json:"idArtist"`
	Name          string            `json:"strArtist"`
	MusicBrainzID string            `json:"strMusicBrainzID"`
	Genre         string            `json:"strGenre"`
	Style         string            `json:"strStyle"`
	Mood          string            `json:"strMood"`
	FormedYear    string            `json:"intFormedYear"`
	BornYear      string            `json:"intBornYear"`
	DiedYear      string            `json:"intDiedYear"`
	Disbanded     string            `json:"strDisbanded"`
	Country       string            `json:"strCountry"`
	Label         string            `json:"strLabel"`
	Thumb         string            `json:"strArtistThumb"`
	Logo          string            `json:"strArtistLogo"`
	Clearart      string            `json:"strArtistClearart"`
	WideThumb     string            `json:"strArtistWideThumb"`
	Banner        string            `json:"strArtistBanner"`
	Fanart        string            `json:"strArtistFanart"`
	Fanart2       string            `json:"strArtistFanart2"`
	Fanart3       string            `json:"strArtistFanart3"`
	Fanart4       string            `json:"strArtistFanart4"`
	Biographies   map[string]string `json:"-"`
}

// UnmarshalJSON also collects the biographies, "strBiographyEN" and the
// like, by their language suffix.
func (a *adbArtist) UnmarshalJSON(data []byte) error {
	type plain adbArtist
	if err := json.Unmarshal(data, (*plain)(a)); err != nil {
		return err
	}
	a.Biographies = texts(data, "strBiography")
	return nil
}

// adbAlbum is an album; empty strings and nulls mean unknown.
type adbAlbum struct {
	ID                  string            `json:"idAlbum"`
	ArtistID            string            `json:"idArtist"`
	Name                string            `json:"strAlbum"`
	Artist              string            `json:"strArtist"`
	YearReleased        string            `json:"intYearReleased"`
	Genre               string            `json:"strGenre"`
	Style               string            `json:"strStyle"`
	Mood                string            `json:"strMood"`
	Label               string            `json:"strLabel"`
	Score               string            `json:"intScore"`
	MusicBrainzID       string            `json:"strMusicBrainzID"`
	MusicBrainzArtistID string            `json:"strMusicBrainzArtistID"`
	Thumb               string            `json:"strAlbumThumb"`
	CDArt               string            `json:"strAlbumCDart"`
	Descriptions        map[string]string `json:"-"`
}

// UnmarshalJSON also collects the descriptions by language.
func (a *adbAlbum) UnmarshalJSON(data []byte) error {
	type plain adbAlbum
	if err := json.Unmarshal(data, (*plain)(a)); err != nil {
		return err
	}
	a.Descriptions = texts(data, "strDescription")
	return nil
}

// texts collects the non-empty string fields named prefix plus a language
// suffix, by the suffix.
func texts(data []byte, prefix string) map[string]string {
	var fields map[string]any
	if json.Unmarshal(data, &fields) != nil {
		return nil
	}
	out := map[string]string{}
	for k, v := range fields {
		s, ok := v.(string)
		if suffix, found := strings.CutPrefix(k, prefix); found && ok && strings.TrimSpace(s) != "" {
			out[suffix] = strings.TrimSpace(s)
		}
	}
	return out
}

func (c *client) artists(ctx context.Context, endpoint string, query url.Values) ([]adbArtist, error) {
	var out struct {
		Artists []adbArtist `json:"artists"`
	}
	return out.Artists, c.get(ctx, endpoint, query, &out)
}

func (c *client) albums(ctx context.Context, endpoint string, query url.Values) ([]adbAlbum, error) {
	var out struct {
		Album []adbAlbum `json:"album"`
	}
	return out.Album, c.get(ctx, endpoint, query, &out)
}
