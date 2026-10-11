package acquire

import (
	"errors"
	"fmt"
	"slices"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// FirstGroupID is the ID of the first group of default profiles; groups
// have IDs from it up, apart from the qualities' own.
const FirstGroupID = 1000

// ProfileItem is a quality, or a group of qualities that rank alike, in
// a quality profile.
type ProfileItem struct {
	// ID is a group's ID, at least FirstGroupID; 0 for a quality.
	ID int
	// Name is a group's name.
	Name    string
	Quality release.Quality
	// Items are a group's qualities.
	Items   []ProfileItem
	Allowed bool
}

// IsGroup reports whether the item is a group.
func (it ProfileItem) IsGroup() bool { return it.ID != 0 || len(it.Items) > 0 }

// FormatScore scores a custom format in a profile.
type FormatScore struct {
	FormatID core.ID
	Score    int
}

// QualityProfile ranks qualities, lowest first, and custom formats: what
// to get of a movie or series and when to stop upgrading it, as Radarr's
// and Sonarr's QualityProfile.
type QualityProfile struct {
	ID   core.ID
	Name string
	// UpgradeAllowed lets a release replace a file until the cutoff.
	UpgradeAllowed bool
	// Cutoff is the ID of the quality or group upgrades stop at.
	Cutoff int
	Items  []ProfileItem
	// MinFormatScore is the least custom format score of a release.
	MinFormatScore int
	// CutoffFormatScore is the custom format score upgrades stop at.
	CutoffFormatScore int
	// MinUpgradeFormatScore is the least increase of the custom format
	// score that is an upgrade.
	MinUpgradeFormatScore int
	FormatItems           []FormatScore
	// Language is the language of movie releases, release.Original for the
	// movie's own; release.LanguageUnknown for any. Series ignore it.
	Language release.Language
}

// Validate checks a profile of a kind as Radarr's API validators do:
// every quality of the kind's definitions in it once, groups of at least
// two named qualities, something allowed and an allowed cutoff.
func (p *QualityProfile) Validate(kind release.Kind) error {
	if p.Name == "" {
		return errors.New("the profile has no name")
	}
	allowed := false
	seen := map[release.Quality]bool{}
	ids := map[int]bool{}
	for _, it := range p.Items {
		if it.Allowed {
			allowed = true
		}
		if it.IsGroup() {
			if it.ID < FirstGroupID {
				return fmt.Errorf("group %q has ID %d, below %d", it.Name, it.ID, FirstGroupID)
			}
			if ids[it.ID] {
				return fmt.Errorf("group ID %d is repeated", it.ID)
			}
			ids[it.ID] = true
			if it.Name == "" {
				return fmt.Errorf("group %d has no name", it.ID)
			}
			if len(it.Items) < 2 {
				return fmt.Errorf("group %q has fewer than two qualities", it.Name)
			}
		}
		for _, q := range it.qualities() {
			if seen[q] {
				return fmt.Errorf("quality %s is repeated", q)
			}
			seen[q] = true
		}
	}
	for _, d := range DefaultDefinitions(kind) {
		if !seen[d.Quality] {
			return fmt.Errorf("quality %s is missing", d.Quality)
		}
	}
	if !allowed {
		return errors.New("the profile allows no quality")
	}
	i := p.itemIndex(p.Cutoff)
	if i < 0 {
		return fmt.Errorf("cutoff %d is not in the profile", p.Cutoff)
	}
	if !p.Items[i].Allowed {
		return errors.New("the cutoff is not allowed")
	}
	return nil
}

func (it ProfileItem) qualities() []release.Quality {
	if !it.IsGroup() {
		return []release.Quality{it.Quality}
	}
	qs := make([]release.Quality, len(it.Items))
	for i, g := range it.Items {
		qs[i] = g.Quality
	}
	return qs
}

// itemIndex returns the index of the item of a quality ID or group ID,
// or -1.
func (p *QualityProfile) itemIndex(id int) int {
	for i, it := range p.Items {
		if it.IsGroup() {
			if it.ID == id {
				return i
			}
			continue
		}
		if QualityID(it.Quality) == id {
			return i
		}
	}
	return -1
}

// FirstAllowedQuality returns the lowest allowed quality.
func (p *QualityProfile) FirstAllowedQuality() release.Quality {
	for _, it := range p.Items {
		if it.Allowed {
			return it.qualities()[0]
		}
	}
	return release.Unknown
}

// LastAllowedQuality returns the highest allowed quality.
func (p *QualityProfile) LastAllowedQuality() release.Quality {
	for _, it := range slices.Backward(p.Items) {
		if it.Allowed {
			qs := it.qualities()
			return qs[len(qs)-1]
		}
	}
	return release.Unknown
}

// QualityIndex places a quality in a profile: the index of its item and
// its index in the item's group.
type QualityIndex struct {
	Index      int
	GroupIndex int
}

// Compare orders indexes, by group index too when respectGroupOrder.
func (x QualityIndex) Compare(o QualityIndex, respectGroupOrder bool) int {
	if c := cmpInt(x.Index, o.Index); c != 0 || !respectGroupOrder {
		return c
	}
	return cmpInt(x.GroupIndex, o.GroupIndex)
}

// Index returns where a quality is in the profile; with
// respectGroupOrder its place in its group too. A quality not in the
// profile is at 0.
func (p *QualityProfile) Index(q release.Quality, respectGroupOrder bool) QualityIndex {
	return p.IndexOf(QualityID(q), respectGroupOrder)
}

// IndexOf is Index of a quality ID or group ID.
func (p *QualityProfile) IndexOf(id int, respectGroupOrder bool) QualityIndex {
	for i, it := range p.Items {
		if !it.IsGroup() && QualityID(it.Quality) == id {
			return QualityIndex{Index: i}
		}
		if it.ID > 0 && it.ID == id {
			return QualityIndex{Index: i}
		}
		for g, gi := range it.Items {
			if QualityID(gi.Quality) == id {
				if respectGroupOrder {
					return QualityIndex{Index: i, GroupIndex: g}
				}
				return QualityIndex{Index: i}
			}
		}
	}
	return QualityIndex{}
}

// Allowed reports whether the profile allows a quality.
func (p *QualityProfile) Allowed(q release.Quality) bool {
	for _, it := range p.Items {
		if slices.Contains(it.qualities(), q) {
			return it.Allowed
		}
	}
	return false
}

// CompareQualities orders two qualities by the profile.
func (p *QualityProfile) CompareQualities(a, b release.Quality, respectGroupOrder bool) int {
	return p.Index(a, respectGroupOrder).Compare(p.Index(b, respectGroupOrder), respectGroupOrder)
}

// Compare orders two quality models by the profile, then by revision, as
// Radarr's QualityModelComparer.
func (p *QualityProfile) Compare(a, b QualityModel, respectGroupOrder bool) int {
	if c := p.CompareQualities(a.Quality, b.Quality, respectGroupOrder); c != 0 {
		return c
	}
	return a.Revision.Compare(b.Revision)
}

// FormatScore returns the sum of the profile's scores of matched formats.
func (p *QualityProfile) FormatScore(formats []CustomFormat) int {
	score := 0
	for _, fi := range p.FormatItems {
		if slices.ContainsFunc(formats, func(f CustomFormat) bool { return f.ID == fi.FormatID }) {
			score += fi.Score
		}
	}
	return score
}

// NewProfile returns a profile of a kind's default definitions, grouped
// by weight, allowing allowed and upgrading until cutoff, as Radarr's
// QualityProfileService.GetDefaultProfile. Every format is scored 0.
func NewProfile(kind release.Kind, name string, cutoff release.Quality, allowed []release.Quality, formats []CustomFormat) QualityProfile {
	defs := DefaultDefinitions(kind)
	var items []ProfileItem
	groupID := FirstGroupID
	cutoffID := QualityID(cutoff)
	for i := 0; i < len(defs); {
		j := i + 1
		for j < len(defs) && defs[j].Weight == defs[i].Weight {
			j++
		}
		group := defs[i:j]
		i = j
		if len(group) == 1 {
			q := group[0].Quality
			items = append(items, ProfileItem{Quality: q, Allowed: slices.Contains(allowed, q)})
			continue
		}
		groupAllowed := slices.ContainsFunc(group, func(d Definition) bool { return slices.Contains(allowed, d.Quality) })
		item := ProfileItem{ID: groupID, Name: group[0].GroupName, Allowed: groupAllowed}
		for _, d := range group {
			item.Items = append(item.Items, ProfileItem{Quality: d.Quality, Allowed: groupAllowed})
			if d.Quality == cutoff {
				cutoffID = groupID
			}
		}
		items = append(items, item)
		groupID++
	}
	p := QualityProfile{
		Name:                  name,
		Cutoff:                cutoffID,
		Items:                 items,
		MinUpgradeFormatScore: 1,
	}
	if kind == release.Movie {
		p.Language = release.Original
	}
	for _, f := range formats {
		p.FormatItems = append(p.FormatItems, FormatScore{FormatID: f.ID})
	}
	return p
}

// DefaultProfiles returns the profiles Radarr (movies) or Sonarr (series)
// creates on first start.
func DefaultProfiles(kind release.Kind, formats []CustomFormat) []QualityProfile {
	type spec struct {
		name    string
		cutoff  release.Quality
		allowed []release.Quality
	}
	var specs []spec
	if kind == release.Series {
		specs = []spec{
			{"Any", release.SDTV, []release.Quality{
				release.SDTV, release.WEBRip480p, release.WEBDL480p, release.DVD, release.Bluray480p,
				release.Bluray576p, release.HDTV720p, release.HDTV1080p, release.WEBRip720p, release.WEBDL720p,
				release.WEBRip1080p, release.WEBDL1080p, release.Bluray720p, release.Bluray1080p,
			}},
			{"SD", release.SDTV, []release.Quality{
				release.SDTV, release.WEBRip480p, release.WEBDL480p, release.DVD, release.Bluray480p, release.Bluray576p,
			}},
			{"HD-720p", release.HDTV720p, []release.Quality{
				release.HDTV720p, release.WEBRip720p, release.WEBDL720p, release.Bluray720p,
			}},
			{"HD-1080p", release.HDTV1080p, []release.Quality{
				release.HDTV1080p, release.WEBRip1080p, release.WEBDL1080p, release.Bluray1080p,
			}},
			{"Ultra-HD", release.HDTV2160p, []release.Quality{
				release.HDTV2160p, release.WEBRip2160p, release.WEBDL2160p, release.Bluray2160p,
			}},
			{"HD - 720p/1080p", release.HDTV720p, []release.Quality{
				release.HDTV720p, release.HDTV1080p, release.WEBRip720p, release.WEBDL720p, release.WEBRip1080p,
				release.WEBDL1080p, release.Bluray720p, release.Bluray1080p,
			}},
		}
	} else {
		specs = []spec{
			{"Any", release.Bluray480p, []release.Quality{
				release.Workprint, release.Cam, release.Telesync, release.Telecine, release.DVDSCR, release.Regional,
				release.SDTV, release.DVD, release.DVDR, release.HDTV720p, release.HDTV1080p, release.HDTV2160p,
				release.WEBDL480p, release.WEBRip480p, release.WEBDL720p, release.WEBRip720p, release.WEBDL1080p,
				release.WEBRip1080p, release.WEBDL2160p, release.WEBRip2160p, release.Bluray480p, release.Bluray576p,
				release.Bluray720p, release.Bluray1080p, release.Bluray2160p, release.Remux1080p, release.Remux2160p,
				release.BRDisk,
			}},
			{"SD", release.Bluray480p, []release.Quality{
				release.Workprint, release.Cam, release.Telesync, release.Telecine, release.DVDSCR, release.Regional,
				release.SDTV, release.DVD, release.WEBDL480p, release.WEBRip480p, release.Bluray480p, release.Bluray576p,
			}},
			{"HD-720p", release.Bluray720p, []release.Quality{
				release.HDTV720p, release.WEBDL720p, release.WEBRip720p, release.Bluray720p,
			}},
			{"HD-1080p", release.Bluray1080p, []release.Quality{
				release.HDTV1080p, release.WEBDL1080p, release.WEBRip1080p, release.Bluray1080p, release.Remux1080p,
			}},
			{"Ultra-HD", release.Remux2160p, []release.Quality{
				release.HDTV2160p, release.WEBDL2160p, release.WEBRip2160p, release.Bluray2160p, release.Remux2160p,
			}},
			{"HD - 720p/1080p", release.Bluray720p, []release.Quality{
				release.HDTV720p, release.HDTV1080p, release.WEBDL720p, release.WEBRip720p, release.WEBDL1080p,
				release.WEBRip1080p, release.Bluray720p, release.Bluray1080p, release.Remux1080p,
			}},
		}
	}
	out := make([]QualityProfile, len(specs))
	for i, s := range specs {
		out[i] = NewProfile(kind, s.name, s.cutoff, s.allowed, formats)
	}
	return out
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// String returns the group's name or the quality's.
func (it ProfileItem) String() string {
	if it.IsGroup() {
		return it.Name
	}
	return it.Quality.String()
}
