package core

import (
	"fmt"
	"slices"
	"time"
)

// User is an account that can sign in and play media: the server's one
// administrator, a member, or a managed user (docs/accounts.md §4.1). Its
// credentials are kept apart, as Credentials.
type User struct {
	ID   ID
	Name string // unique, case-insensitive
	Kind UserKind
	// AuthProvider names the plugin that authenticates the user, such as
	// LDAP; empty means the server's own credentials.
	AuthProvider string
	// AccountID is the Mavio account of a user of a server in mavio.dev
	// mode, unique; empty in standalone mode.
	AccountID string
	// Restriction is a managed user's restriction profile, applied on top
	// of its library access.
	Restriction RestrictionProfile
	Disabled    bool
	Access      LibraryAccess
	Policy      UserPolicy
	Preferences PlaybackPreferences
	Home        HomeLayout
	CreatedAt   time.Time
	LastLoginAt *time.Time
}

// UserKind is what a user is to the server.
type UserKind string

// User kinds.
const (
	// UserMember signs in with its own credentials.
	UserMember UserKind = "member"
	// UserAdmin is the server's one administrator, a member that manages
	// it.
	UserAdmin UserKind = "administrator"
	// UserManaged has no credentials and is reached only by switching to
	// it; giving it a credential makes it a member that keeps its data.
	UserManaged UserKind = "managed"
)

// Valid reports whether k is a known user kind.
func (k UserKind) Valid() bool {
	return slices.Contains([]UserKind{UserMember, UserAdmin, UserManaged}, k)
}

// IsAdmin reports whether u is the server's administrator.
func (u *User) IsAdmin() bool { return u.Kind == UserAdmin }

// Validate checks the user's invariants.
func (u *User) Validate() error {
	switch {
	case u.ID.IsZero() || u.Name == "":
		return fmt.Errorf("%w: user requires ID and name", ErrInvalid)
	case !u.Kind.Valid():
		return fmt.Errorf("%w: user %q has unknown kind %q", ErrInvalid, u.Name, u.Kind)
	case u.Kind == UserManaged && u.AuthProvider != "":
		return fmt.Errorf("%w: managed user %q has an auth provider", ErrInvalid, u.Name)
	case u.AccountID != "" && u.AuthProvider != "":
		return fmt.Errorf("%w: user %q of a Mavio account has an auth provider", ErrInvalid, u.Name)
	case !u.Restriction.Valid():
		return fmt.Errorf("%w: user %q has unknown restriction profile %q", ErrInvalid, u.Name, u.Restriction)
	case u.Restriction != RestrictionNone && u.Kind != UserManaged:
		return fmt.Errorf("%w: user %q has a restriction profile but is not managed", ErrInvalid, u.Name)
	case !u.Preferences.SubtitleMode.Valid():
		return fmt.Errorf("%w: user %q has unknown subtitle mode %q", ErrInvalid, u.Name, u.Preferences.SubtitleMode)
	case u.Policy.MovieQuota != nil && u.Policy.MovieQuota.Validate() != nil,
		u.Policy.SeriesQuota != nil && u.Policy.SeriesQuota.Validate() != nil:
		return fmt.Errorf("%w: user %q has a negative request quota", ErrInvalid, u.Name)
	}
	return u.Access.Validate()
}

// Credential is what a user signs in or switches with in standalone mode:
// a password, a PIN, a TOTP secret or a passkey. What is stored lets
// nobody sign in from a copy of the database: hashes, public keys, and
// TOTP secrets sealed with a key derived from the server's identity key.
type Credential struct {
	ID     ID
	UserID ID
	Kind   CredentialKind
	// Name tells a user's passkeys apart; empty for other kinds.
	Name string
	// Data is a PHC-format hash (e.g. "$argon2id$v=19$…") for passwords
	// and PINs, the sealed secret for TOTP, and the public key credential
	// for passkeys.
	Data       string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// CredentialKind is the kind of a credential.
type CredentialKind string

// Credential kinds.
const (
	// CredentialPassword signs a member in.
	CredentialPassword CredentialKind = "password"
	// CredentialPIN is four digits guarding switching to the user; never a
	// credential for signing in.
	CredentialPIN CredentialKind = "pin"
	// CredentialTOTP is the secret of time-based one-time passwords (RFC
	// 6238), a second factor to a password.
	CredentialTOTP CredentialKind = "totp"
	// CredentialPasskey is a WebAuthn credential, which signs a member in
	// alone; a user may have several.
	CredentialPasskey CredentialKind = "passkey"
)

// Valid reports whether k is a known credential kind.
func (k CredentialKind) Valid() bool {
	return slices.Contains([]CredentialKind{CredentialPassword, CredentialPIN, CredentialTOTP, CredentialPasskey}, k)
}

// Single reports whether a user has at most one credential of kind k:
// every kind but passkeys.
func (k CredentialKind) Single() bool { return k != CredentialPasskey }

// Validate checks the credential's invariants.
func (c *Credential) Validate() error {
	switch {
	case c.ID.IsZero() || c.UserID.IsZero():
		return fmt.Errorf("%w: credential requires ID and user", ErrInvalid)
	case !c.Kind.Valid():
		return fmt.Errorf("%w: unknown credential kind %q", ErrInvalid, c.Kind)
	case c.Data == "":
		return fmt.Errorf("%w: %s credential without data", ErrInvalid, c.Kind)
	}
	return nil
}

// UserPolicy is what a user may do on the server beyond its library
// access: transcoding, streaming and requests.
type UserPolicy struct {
	AllowTranscoding bool
	// MaxStreamingBitrate caps remote streaming, in bits per second; zero
	// means unlimited.
	MaxStreamingBitrate int64
	// MaxSessions limits concurrent playback sessions; zero means unlimited.
	MaxSessions int
	// Requests are what the user may request and report; administrators
	// may do all.
	Requests RequestPermissions
	// MovieQuota and SeriesQuota bound the user's requests; nil uses the
	// server's RequestSettings.
	MovieQuota, SeriesQuota *Quota
}

// CanAccess reports whether the user may see the item: their own
// playlists, and other items as their access allows.
func (u *User) CanAccess(it *Item) bool {
	if it.Kind == KindPlaylist {
		return it.UserID == u.ID
	}
	access := u.EffectiveAccess()
	return access.CanAccess(it)
}

// EffectiveAccess returns the user's library access record with its
// restriction profile's rating limit applied.
func (u *User) EffectiveAccess() LibraryAccess {
	a := u.Access
	if limit, ok := u.Restriction.MaxRating(); ok {
		if a.MaxParentalRating == nil || *a.MaxParentalRating > limit {
			a.MaxParentalRating = &limit
		}
		a.BlockUnrated = true
	}
	return a
}

// RestrictionProfile limits what a managed user sees by content rating,
// after Plex's profiles.
type RestrictionProfile string

// Restriction profiles.
const (
	RestrictionNone         RestrictionProfile = ""
	RestrictionYoungerChild RestrictionProfile = "younger_child"
	RestrictionOlderChild   RestrictionProfile = "older_child"
	RestrictionTeen         RestrictionProfile = "teen"
)

// Valid reports whether r is a known restriction profile.
func (r RestrictionProfile) Valid() bool {
	return slices.Contains([]RestrictionProfile{RestrictionNone, RestrictionYoungerChild, RestrictionOlderChild, RestrictionTeen}, r)
}

// MaxRating returns the highest rating score, an age, the profile allows;
// false for none. Rated items within it alone are shown: up to G and TV-G
// for younger children, PG and TV-PG for older children, PG-13 and TV-14
// for teenagers.
func (r RestrictionProfile) MaxRating() (int, bool) {
	switch r {
	case RestrictionYoungerChild:
		return 6, true
	case RestrictionOlderChild:
		return 12, true
	case RestrictionTeen:
		return 16, true
	}
	return 0, false
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

// PlaybackPreferences are playback defaults chosen by the user.
type PlaybackPreferences struct {
	// AudioLanguages and SubtitleLanguages are ISO 639-2/B codes in order of
	// preference.
	AudioLanguages    []string
	SubtitleLanguages []string
	SubtitleMode      SubtitleMode
	// PlayDefaultAudioTrack prefers the stream flagged default over the
	// preferred language.
	PlayDefaultAudioTrack bool
}

// HomeLayout is a user's choice of what Home shows.
type HomeLayout struct {
	// Pinned reports whether the user chose Home's libraries; when not,
	// Home shows every library the user may access, in name order.
	Pinned bool
	// Libraries are the pinned libraries, in the order Home shows them.
	Libraries []ID
	// HiddenHubs are the IDs of the hubs the user hid.
	HiddenHubs []string
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
