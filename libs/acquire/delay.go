package acquire

import (
	"math"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Protocol is how a release is downloaded.
type Protocol int

// Protocols.
const (
	ProtocolUnknown Protocol = iota
	ProtocolUsenet
	ProtocolTorrent
)

// DelayProfile holds releases back for a while so that better ones can
// turn up, as Radarr's and Sonarr's DelayProfile.
type DelayProfile struct {
	ID core.ID
	// Default is set on the profile that applies to every tag and comes
	// last, whatever its Order.
	Default           bool
	EnableUsenet      bool
	EnableTorrent     bool
	PreferredProtocol Protocol
	UsenetDelay       time.Duration
	TorrentDelay      time.Duration
	// Order ranks the profiles, lowest first; the default profile comes
	// after all others.
	Order int
	// BypassIfHighestQuality grabs a release of the profile's highest
	// allowed quality at once.
	BypassIfHighestQuality bool
	// BypassIfAboveCustomFormatScore grabs a release whose custom format
	// score reaches MinimumCustomFormatScore at once.
	BypassIfAboveCustomFormatScore bool
	MinimumCustomFormatScore       int
	// Tags are the tags of the movies and series the profile applies to,
	// all when none.
	Tags []string
}

// Delay returns the profile's delay of a protocol.
func (p *DelayProfile) Delay(protocol Protocol) time.Duration {
	if protocol == ProtocolTorrent {
		return p.TorrentDelay
	}
	return p.UsenetDelay
}

// appliesTo reports whether the profile applies to an item of tags.
func (p *DelayProfile) appliesTo(tags []string) bool {
	return p.Default || len(p.Tags) == 0 || slices.ContainsFunc(p.Tags, func(t string) bool { return slices.Contains(tags, t) })
}

// rank is the profile's order, the default profile's last.
func (p *DelayProfile) rank() int {
	if p.Default {
		return math.MaxInt
	}
	return p.Order
}

// BestDelayProfile returns the first profile, by order, that applies to an
// item of tags, as DelayProfileService.BestForTags; nil when none.
func BestDelayProfile(profiles []DelayProfile, tags []string) *DelayProfile {
	var best *DelayProfile
	for i := range profiles {
		p := &profiles[i]
		if p.appliesTo(tags) && (best == nil || p.rank() < best.rank()) {
			best = p
		}
	}
	return best
}

// ReorderDelayProfiles moves the profile of id after the profile of
// afterID, or first when afterID is zero, renumbering the others as
// DelayProfileService.Reorder; the default profile stays last. It reports
// false when there is no profile of id.
func ReorderDelayProfiles(profiles []DelayProfile, id, afterID core.ID) bool {
	slices.SortStableFunc(profiles, func(a, b DelayProfile) int { return cmpInt(a.rank(), b.rank()) })
	mi := slices.IndexFunc(profiles, func(p DelayProfile) bool { return p.ID == id })
	if mi < 0 {
		return false
	}
	moving := profiles[mi]
	afterOrder := 0
	if ai := slices.IndexFunc(profiles, func(p DelayProfile) bool { return !afterID.IsZero() && p.ID == afterID }); ai >= 0 {
		afterOrder = profiles[ai].Order
		if moving.Order < afterOrder {
			afterOrder--
		}
	}
	afterCount := afterOrder + 2
	for i := range profiles {
		p := &profiles[i]
		switch {
		case p.Default:
		case p.ID == id:
			p.Order = afterOrder + 1
		case !afterID.IsZero() && p.ID == afterID:
			p.Order = afterOrder
		case p.Order > afterOrder:
			p.Order = afterCount
			afterCount++
		case p.Order > moving.Order:
			p.Order--
		}
	}
	return true
}
