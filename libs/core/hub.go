package core

import (
	"fmt"
	"slices"
)

// HubKind is a kind of hub: a row of items on Home or on a library's
// Recommended tab, each a reason to watch something, as Plex's hubs.
type HubKind string

// Hub kinds. Available requests and trending span the libraries and appear
// on Home only; continue watching spans them on Home and belongs to a
// library on its tab; the others belong to a library.
const (
	// HubContinueWatching lists the items being watched and the next
	// episodes, by last activity.
	HubContinueWatching HubKind = "continue_watching"
	HubRecentlyAdded    HubKind = "recently_added"
	// HubRecentlyReleased lists items by release date; HubRecentlyAired
	// lists episodes by air date.
	HubRecentlyReleased HubKind = "recently_released"
	HubRecentlyAired    HubKind = "recently_aired"
	HubTopRated         HubKind = "top_rated"
	// HubTopUnplayed lists the best rated items the user has not played.
	HubTopUnplayed HubKind = "top_unplayed"
	// HubStartWatching lists the series the user has not begun.
	HubStartWatching HubKind = "start_watching"
	// HubRediscover lists what the user played long ago: items last played
	// over a year ago, series last watched over half a year ago.
	HubRediscover HubKind = "rediscover"
	// The seeded hubs: items like one the user played recently or likes,
	// and items of a genre, actor or director of what they played.
	HubBecauseYouWatched HubKind = "because_you_watched"
	HubBecauseYouLike    HubKind = "because_you_like"
	HubMoreInGenre       HubKind = "more_in_genre"
	HubStarring          HubKind = "starring"
	HubDirectedBy        HubKind = "directed_by"
	// HubAvailableRequests lists the items the user requested that became
	// available.
	HubAvailableRequests HubKind = "available_requests"
	// HubTrending lists the titles discovery offers that the server lacks.
	HubTrending HubKind = "trending"
)

// HubKinds lists every hub kind.
var HubKinds = []HubKind{
	HubContinueWatching, HubRecentlyAdded, HubRecentlyReleased, HubRecentlyAired,
	HubTopRated, HubTopUnplayed, HubStartWatching, HubRediscover,
	HubBecauseYouWatched, HubBecauseYouLike, HubMoreInGenre, HubStarring, HubDirectedBy,
	HubAvailableRequests, HubTrending,
}

// Valid reports whether k is a known hub kind.
func (k HubKind) Valid() bool { return slices.Contains(HubKinds, k) }

// Global reports whether hubs of the kind span the libraries.
func (k HubKind) Global() bool {
	return k == HubAvailableRequests || k == HubTrending
}

// Seeded reports whether hubs of the kind are drawn from a seed: an item,
// a genre or a person.
func (k HubKind) Seeded() bool {
	switch k {
	case HubBecauseYouWatched, HubBecauseYouLike, HubMoreInGenre, HubStarring, HubDirectedBy:
		return true
	}
	return false
}

// HubKinds lists the hub kinds a library of kind k offers, in their default
// order on its Recommended tab.
func (k LibraryKind) HubKinds() []HubKind {
	switch k {
	case LibraryMovies:
		return []HubKind{
			HubContinueWatching, HubRecentlyAdded, HubRecentlyReleased, HubTopRated, HubTopUnplayed,
			HubBecauseYouWatched, HubBecauseYouLike, HubMoreInGenre, HubStarring, HubDirectedBy, HubRediscover,
		}
	case LibraryShows:
		return []HubKind{
			HubContinueWatching, HubRecentlyAdded, HubRecentlyAired, HubTopRated, HubStartWatching,
			HubBecauseYouWatched, HubBecauseYouLike, HubMoreInGenre, HubStarring, HubRediscover,
		}
	case LibraryMusicVideos, LibraryHomeVideos, LibraryMixed:
		return []HubKind{HubContinueWatching, HubRecentlyAdded, HubRecentlyReleased, HubTopRated}
	case LibraryMusic, LibraryBooks, LibraryPhotos:
		return []HubKind{HubRecentlyAdded, HubRecentlyReleased}
	}
	return nil
}

// defaultHomeHubs are the hubs of a library of kind k shown on Home by
// default.
func (k LibraryKind) defaultHomeHubs() []HubKind {
	switch k {
	case LibraryMovies:
		return []HubKind{HubRecentlyAdded, HubBecauseYouWatched, HubTopUnplayed}
	case LibraryShows:
		return []HubKind{HubRecentlyAdded, HubStartWatching}
	case LibraryCollections, LibraryPlaylists:
		return nil
	}
	return []HubKind{HubRecentlyAdded}
}

// LibraryHubs are the hubs an administrator chose for a library, in order:
// those on Home and those on its Recommended tab. A nil list keeps the
// defaults of the library's kind; an empty one shows none. Continue
// watching on Home spans the libraries and is not listed among a library's
// Home hubs.
type LibraryHubs struct {
	Home        []HubKind `json:"home"`
	Recommended []HubKind `json:"recommended"`
}

// Resolve returns the hubs of a library of kind k on Home and on its
// Recommended tab, applying the defaults.
func (h LibraryHubs) Resolve(k LibraryKind) (home, recommended []HubKind) {
	home, recommended = h.Home, h.Recommended
	if home == nil {
		home = k.defaultHomeHubs()
	}
	if recommended == nil {
		recommended = k.HubKinds()
	}
	return home, recommended
}

func (h LibraryHubs) validate(k LibraryKind) error {
	offered := k.HubKinds()
	for _, list := range [][]HubKind{h.Home, h.Recommended} {
		for i, hub := range list {
			switch {
			case !slices.Contains(offered, hub):
				return fmt.Errorf("%w: %s libraries have no %q hub", ErrInvalid, k, hub)
			case slices.Contains(list[:i], hub):
				return fmt.Errorf("%w: hub %q listed twice", ErrInvalid, hub)
			}
		}
	}
	if slices.Contains(h.Home, HubContinueWatching) {
		return fmt.Errorf("%w: continue watching on Home spans the libraries", ErrInvalid)
	}
	return nil
}
