package core

import (
	"fmt"
	"slices"
	"time"
)

// User is an account that can sign in and play media.
type User struct {
	ID   ID
	Name string // unique, case-insensitive
	// PasswordHash is a PHC-format string (e.g. "$argon2id$v=19$…"); empty for
	// accounts that authenticate through a plugin such as LDAP.
	PasswordHash string
	// AuthProvider names the plugin that authenticates the user; empty means
	// the built-in password check.
	AuthProvider string
	Admin        bool
	Disabled     bool
	Policy       UserPolicy
	Preferences  UserPreferences
	CreatedAt    time.Time
	LastLoginAt  *time.Time
}

// Validate checks the user's invariants.
func (u *User) Validate() error {
	switch {
	case u.ID.IsZero() || u.Name == "":
		return fmt.Errorf("%w: user requires ID and name", ErrInvalid)
	case u.PasswordHash == "" && u.AuthProvider == "":
		return fmt.Errorf("%w: user %q has neither a password nor an auth provider", ErrInvalid, u.Name)
	case !u.Preferences.SubtitleMode.Valid():
		return fmt.Errorf("%w: user %q has unknown subtitle mode %q", ErrInvalid, u.Name, u.Preferences.SubtitleMode)
	}
	return nil
}

// UserPolicy restricts what a user may access and do.
type UserPolicy struct {
	// Libraries limits access to these libraries; nil means all libraries.
	Libraries []ID
	// MaxParentalRating is the highest allowed rating score; content ratings
	// such as "PG-13" are mapped to scores per country rating system. Nil
	// means unrestricted; zero allows only content for all ages.
	MaxParentalRating *int
	// BlockUnrated hides items without an official rating when a maximum
	// rating is set.
	BlockUnrated     bool
	AllowTranscoding bool
	AllowDownload    bool
	// MaxStreamingBitrate caps remote streaming, in bits per second; zero
	// means unlimited.
	MaxStreamingBitrate int64
	// MaxSessions limits concurrent playback sessions; zero means unlimited.
	MaxSessions int
}

// CanAccessLibrary reports whether the policy allows the library.
func (p *UserPolicy) CanAccessLibrary(id ID) bool {
	return p.Libraries == nil || slices.Contains(p.Libraries, id)
}

// CanAccess reports whether the policy allows the item: its library is
// allowed and, under a maximum rating, its rating score is within it,
// unrated items only when BlockUnrated is off. The score is the item's own
// ParentalRating, or its InheritedRating when it has none.
func (p *UserPolicy) CanAccess(it *Item) bool {
	if !p.CanAccessLibrary(it.LibraryID) {
		return false
	}
	if p.MaxParentalRating != nil {
		score := it.ParentalRating
		if score == nil {
			score = it.InheritedRating
		}
		if score == nil {
			return !p.BlockUnrated
		}
		return *score <= *p.MaxParentalRating
	}
	return true
}

// CanAccess reports whether the user may see the item: their own
// playlists, and other items as their policy allows.
func (u *User) CanAccess(it *Item) bool {
	if it.Kind == KindPlaylist {
		return it.UserID == u.ID
	}
	return u.Policy.CanAccess(it)
}

// SubtitleMode controls automatic subtitle selection.
type SubtitleMode string

// Subtitle modes.
const (
	SubtitlesDefault    SubtitleMode = ""        // follow stream flags
	SubtitlesAlways     SubtitleMode = "always"  // pick a subtitle in the preferred language
	SubtitlesForeign    SubtitleMode = "foreign" // only when the audio is not in the preferred language
	SubtitlesForcedOnly SubtitleMode = "forced"  // only forced subtitles
	SubtitlesNone       SubtitleMode = "none"    // never
	SubtitlesSmart      SubtitleMode = "smart"   // foreign, plus forced in the preferred language
)

// Valid reports whether m is a known subtitle mode.
func (m SubtitleMode) Valid() bool {
	return slices.Contains([]SubtitleMode{SubtitlesDefault, SubtitlesAlways, SubtitlesForeign, SubtitlesForcedOnly, SubtitlesNone, SubtitlesSmart}, m)
}

// UserPreferences are playback defaults chosen by the user.
type UserPreferences struct {
	// AudioLanguages and SubtitleLanguages are ISO 639-2/B codes in order of
	// preference.
	AudioLanguages    []string
	SubtitleLanguages []string
	SubtitleMode      SubtitleMode
	// PlayDefaultAudioTrack prefers the stream flagged default over the
	// preferred language.
	PlayDefaultAudioTrack bool
}

// UserData is one user's state for one item.
type UserData struct {
	UserID ID
	ItemID ID
	Played bool
	// PlayCount counts completed playbacks.
	PlayCount int
	// Position is the resume position; zero means start from the beginning.
	Position time.Duration
	// AudioStream and SubtitleStream remember the last selected stream
	// indexes; nil means no remembered choice, -1 for subtitles means off.
	AudioStream    *int
	SubtitleStream *int
	Favorite       bool
	Rating         *float64 // user rating, 0–10
	LastPlayedAt   *time.Time
	UpdatedAt      time.Time
}

// Validate checks the user data's invariants.
func (d *UserData) Validate() error {
	switch {
	case d.UserID.IsZero() || d.ItemID.IsZero():
		return fmt.Errorf("%w: user data requires user and item", ErrInvalid)
	case d.PlayCount < 0 || d.Position < 0:
		return fmt.Errorf("%w: negative play count or position", ErrInvalid)
	case d.Rating != nil && (*d.Rating < 0 || *d.Rating > 10):
		return fmt.Errorf("%w: user rating %v out of range", ErrInvalid, *d.Rating)
	}
	return nil
}
