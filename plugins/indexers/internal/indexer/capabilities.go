package indexer

import (
	"fmt"
	"slices"
	"strings"
)

// Param is a search parameter, in Newznab's name.
type Param string

// The search parameters.
const (
	ParamQ         Param = "q"
	ParamSeason    Param = "season"
	ParamEp        Param = "ep"
	ParamIMDbID    Param = "imdbid"
	ParamTVDBID    Param = "tvdbid"
	ParamRID       Param = "rid"
	ParamTVMazeID  Param = "tvmazeid"
	ParamTraktID   Param = "traktid"
	ParamTMDBID    Param = "tmdbid"
	ParamDoubanID  Param = "doubanid"
	ParamGenre     Param = "genre"
	ParamYear      Param = "year"
	ParamIMDbTitle Param = "imdbtitle"
	ParamIMDbYear  Param = "imdbyear"
	ParamAlbum     Param = "album"
	ParamArtist    Param = "artist"
	ParamLabel     Param = "label"
	ParamTrack     Param = "track"
	ParamTitle     Param = "title"
	ParamAuthor    Param = "author"
	ParamPublisher Param = "publisher"
)

// The parameters each search type takes, in the order Newznab's caps
// list them.
var searchTypeParams = map[SearchType][]Param{
	SearchBasic: {ParamQ},
	SearchTV:    {ParamQ, ParamSeason, ParamEp, ParamIMDbID, ParamTVDBID, ParamRID, ParamTVMazeID, ParamTraktID, ParamTMDBID, ParamDoubanID, ParamGenre, ParamYear},
	SearchMovie: {ParamQ, ParamIMDbID, ParamTMDBID, ParamIMDbTitle, ParamIMDbYear, ParamTraktID, ParamGenre, ParamDoubanID, ParamYear},
	SearchMusic: {ParamQ, ParamAlbum, ParamArtist, ParamLabel, ParamYear, ParamGenre, ParamTrack},
	SearchBook:  {ParamQ, ParamTitle, ParamAuthor, ParamPublisher, ParamGenre, ParamYear},
}

// Flag is a property of a release, such as freeleech.
type Flag string

// The flags Prowlarr knows.
const (
	FlagInternal     Flag = "internal"
	FlagExclusive    Flag = "exclusive"
	FlagFreeleech    Flag = "freeleech"
	FlagNeutralLeech Flag = "neutralleech"
	FlagHalfLeech    Flag = "halfleech"
	FlagScene        Flag = "scene"
	FlagDoubleUpload Flag = "doubleupload"
)

// Capabilities are what an indexer can be searched by and the categories
// it has.
type Capabilities struct {
	// The number of results a search returns by default and at most.
	LimitsDefault, LimitsMax int
	// The indexer takes queries as given.
	RawSearch bool
	// The parameters of each search type; none when the type is not
	// searchable.
	Params     map[SearchType][]Param
	Categories Categories
	Flags      []Flag
}

// NewCapabilities returns the capabilities of an indexer that takes only
// basic searches, 100 results at a time.
func NewCapabilities() *Capabilities {
	return &Capabilities{
		LimitsDefault: 100, LimitsMax: 100,
		Params: map[SearchType][]Param{SearchBasic: {ParamQ}},
	}
}

// Available reports whether the search type is searchable.
func (c *Capabilities) Available(t SearchType) bool {
	return len(c.Params[t]) > 0
}

// Has reports whether the search type takes the parameter.
func (c *Capabilities) Has(t SearchType, p Param) bool {
	return slices.Contains(c.Params[t], p)
}

// SupportedParams lists the parameters a search type takes as Newznab's
// caps do: q, then the others in their usual order.
func (c *Capabilities) SupportedParams(t SearchType) []Param {
	out := []Param{ParamQ}
	for _, p := range searchTypeParams[t] {
		// Newznab's caps leave out the IMDb title and year.
		if p != ParamQ && p != ParamIMDbTitle && p != ParamIMDbYear && c.Has(t, p) {
			out = append(out, p)
		}
	}
	return out
}

// AddParams adds parameters, given by name in any case, to a search type,
// failing on unknown or repeated ones.
func (c *Capabilities) AddParams(t SearchType, names []string) error {
	if c.Params == nil {
		c.Params = map[SearchType][]Param{}
	}
	for _, name := range names {
		p := Param(strings.ToLower(name))
		if !slices.Contains(searchTypeParams[t], p) {
			return fmt.Errorf("%s search does not take %s", t, name)
		}
		if c.Has(t, p) {
			return fmt.Errorf("%s search has %s twice", t, name)
		}
		c.Params[t] = append(c.Params[t], p)
	}
	return nil
}

// Merge adds another indexer's parameters and standard categories, for
// searching several as one.
func (c *Capabilities) Merge(other *Capabilities) {
	for t, ps := range other.Params {
		for _, p := range ps {
			if !c.Has(t, p) {
				c.Params[t] = append(c.Params[t], p)
			}
		}
	}
	c.Categories.Merge(&other.Categories)
}
