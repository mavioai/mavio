package metadata

import (
	"net/url"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// ExternalIDKind is a kind of external ID items or persons carry, such as
// the ID of a movie on a metadata site.
type ExternalIDKind struct {
	// Provider is the key of the IDs in ExternalIDs maps.
	Provider core.Provider
	// Name is the display name, e.g. "TMDB".
	Name string
	// Items maps the kinds of items that carry the ID to the template of
	// their pages, "{id}" standing for the ID; "" when there is no page.
	Items map[core.ItemKind]string
	// Persons reports whether persons carry the ID, and PersonURL is the
	// template of their pages.
	Persons   bool
	PersonURL string
	// Plugin is the ID of the plugin declaring the kind; empty for
	// built-in kinds.
	Plugin string
}

// ExternalURL is the page of an item or person on an external site.
type ExternalURL struct {
	Name, URL string
}

const (
	tmdbURL        = "https://www.themoviedb.org"
	imdbURL        = "https://www.imdb.com"
	tvdbURL        = "https://www.thetvdb.com/dereferrer"
	musicBrainzURL = "https://musicbrainz.org"
)

// BuiltinExternalIDKinds returns the kinds of external IDs the server's own
// providers and NFO files give.
func BuiltinExternalIDKinds() []ExternalIDKind {
	anime := func(base string) map[core.ItemKind]string {
		return map[core.ItemKind]string{core.KindSeries: base, core.KindSeason: base, core.KindMovie: base}
	}
	return []ExternalIDKind{
		{Provider: core.ProviderTMDB, Name: "TMDB", Items: map[core.ItemKind]string{
			core.KindMovie:      tmdbURL + "/movie/{id}",
			core.KindSeries:     tmdbURL + "/tv/{id}",
			core.KindCollection: tmdbURL + "/collection/{id}",
		}, Persons: true, PersonURL: tmdbURL + "/person/{id}"},
		{Provider: core.ProviderTMDBCollection, Name: "TMDB Collection", Items: map[core.ItemKind]string{
			core.KindMovie: tmdbURL + "/collection/{id}",
		}},
		{Provider: core.ProviderIMDb, Name: "IMDb", Items: map[core.ItemKind]string{
			core.KindMovie:      imdbURL + "/title/{id}",
			core.KindSeries:     imdbURL + "/title/{id}",
			core.KindEpisode:    imdbURL + "/title/{id}",
			core.KindMusicVideo: imdbURL + "/title/{id}",
		}, Persons: true, PersonURL: imdbURL + "/name/{id}"},
		{Provider: core.ProviderTVDB, Name: "TheTVDB", Items: map[core.ItemKind]string{
			core.KindMovie:   tvdbURL + "/movie/{id}",
			core.KindSeries:  tvdbURL + "/series/{id}",
			core.KindEpisode: tvdbURL + "/episode/{id}",
		}},
		{Provider: core.ProviderTVMaze, Name: "TVmaze", Items: map[core.ItemKind]string{
			core.KindSeries:  "https://www.tvmaze.com/shows/{id}",
			core.KindEpisode: "https://www.tvmaze.com/episodes/{id}",
		}},
		{Provider: core.ProviderAniDB, Name: "AniDB", Items: anime("https://anidb.net/anime/{id}")},
		{Provider: core.ProviderAniList, Name: "AniList", Items: anime("https://anilist.co/anime/{id}")},
		{Provider: core.ProviderAniSearch, Name: "aniSearch", Items: anime("https://www.anisearch.com/anime/{id}")},
		{Provider: core.ProviderMusicBrainzArtist, Name: "MusicBrainz Artist", Items: map[core.ItemKind]string{
			core.KindMusicArtist: musicBrainzURL + "/artist/{id}",
		}},
		{Provider: core.ProviderMusicBrainzAlbumArtist, Name: "MusicBrainz Album Artist", Items: map[core.ItemKind]string{
			core.KindMusicAlbum: musicBrainzURL + "/artist/{id}",
			core.KindTrack:      musicBrainzURL + "/artist/{id}",
		}},
		{Provider: core.ProviderMusicBrainzAlbum, Name: "MusicBrainz Release", Items: map[core.ItemKind]string{
			core.KindMusicAlbum: musicBrainzURL + "/release/{id}",
			core.KindTrack:      musicBrainzURL + "/release/{id}",
		}},
		{Provider: core.ProviderMusicBrainzReleaseGroup, Name: "MusicBrainz Release Group", Items: map[core.ItemKind]string{
			core.KindMusicAlbum: musicBrainzURL + "/release-group/{id}",
			core.KindTrack:      musicBrainzURL + "/release-group/{id}",
		}},
		{Provider: core.ProviderMusicBrainzTrack, Name: "MusicBrainz Track", Items: map[core.ItemKind]string{
			core.KindTrack: musicBrainzURL + "/track/{id}",
		}},
	}
}

// MergeExternalIDKinds appends to the built-in kinds those of plugins, in
// order, leaving out kinds whose provider is already listed.
func MergeExternalIDKinds(builtin []ExternalIDKind, plugins ...ExternalIDKind) []ExternalIDKind {
	out := slices.Clone(builtin)
	for _, k := range plugins {
		if !slices.ContainsFunc(out, func(o ExternalIDKind) bool { return o.Provider == k.Provider }) {
			out = append(out, k)
		}
	}
	return out
}

// ItemExternalURLs returns the pages of an item of a kind with external
// IDs, in the order of kinds.
func ItemExternalURLs(kinds []ExternalIDKind, kind core.ItemKind, ids map[core.Provider]string) []ExternalURL {
	var out []ExternalURL
	for _, k := range kinds {
		if t := k.Items[kind]; t != "" && ids[k.Provider] != "" {
			out = append(out, ExternalURL{Name: k.Name, URL: expand(t, ids[k.Provider])})
		}
	}
	return out
}

// PersonExternalURLs returns the pages of a person with external IDs, in
// the order of kinds.
func PersonExternalURLs(kinds []ExternalIDKind, ids map[core.Provider]string) []ExternalURL {
	var out []ExternalURL
	for _, k := range kinds {
		if k.Persons && k.PersonURL != "" && ids[k.Provider] != "" {
			out = append(out, ExternalURL{Name: k.Name, URL: expand(k.PersonURL, ids[k.Provider])})
		}
	}
	return out
}

// expand puts an ID, escaped as a path segment, into a URL template.
func expand(template, id string) string {
	return strings.ReplaceAll(template, "{id}", url.PathEscape(id))
}
