package core

import (
	"fmt"
	"time"
)

// Limits of display preferences.
const (
	MaxDisplayValues     = 200
	MaxDisplayKeyLength  = 200
	MaxDisplayValueBytes = 8 << 10
)

// DisplayPreferences are a user's settings for how one client shows one
// view, such as the sort order of a library or the sections of the home
// screen. The server keeps them so they follow the user across devices;
// their names and values are the client's.
type DisplayPreferences struct {
	UserID ID
	// Client names the client application, e.g. "mavio-web".
	Client string
	// View names what the settings are for, e.g. "home" or a library ID.
	View      string
	Values    map[string]string
	UpdatedAt time.Time
}

// Validate checks the preferences' invariants and limits.
func (p *DisplayPreferences) Validate() error {
	switch {
	case p.UserID.IsZero():
		return fmt.Errorf("%w: display preferences require a user", ErrInvalid)
	case p.Client == "" || len(p.Client) > MaxDisplayKeyLength:
		return fmt.Errorf("%w: display preferences require a client name of up to %d bytes", ErrInvalid, MaxDisplayKeyLength)
	case p.View == "" || len(p.View) > MaxDisplayKeyLength:
		return fmt.Errorf("%w: display preferences require a view name of up to %d bytes", ErrInvalid, MaxDisplayKeyLength)
	case len(p.Values) > MaxDisplayValues:
		return fmt.Errorf("%w: more than %d display preferences", ErrInvalid, MaxDisplayValues)
	}
	for k, v := range p.Values {
		if k == "" || len(k) > MaxDisplayKeyLength || len(v) > MaxDisplayValueBytes {
			return fmt.Errorf("%w: display preference %q exceeds the limits", ErrInvalid, k)
		}
	}
	return nil
}
