package core

import (
	"fmt"
	"time"
)

// AuthSession is a signed-in client: the access token issued to one
// device of a user. Only the token's SHA-256 hash is stored.
type AuthSession struct {
	ID        ID
	UserID    ID
	TokenHash []byte
	// DeviceID identifies the client installation; signing in again from
	// the same device replaces its session.
	DeviceID   string
	DeviceName string
	// Client and ClientVersion name the application, e.g. "Mavio Web".
	Client, ClientVersion string
	CreatedAt             time.Time
	LastSeenAt            time.Time
}

// Validate checks the session's invariants.
func (s *AuthSession) Validate() error {
	switch {
	case s.ID.IsZero() || s.UserID.IsZero():
		return fmt.Errorf("%w: auth session requires ID and user", ErrInvalid)
	case len(s.TokenHash) != 32:
		return fmt.Errorf("%w: auth session %s requires a SHA-256 token hash", ErrInvalid, s.ID)
	case s.DeviceID == "":
		return fmt.Errorf("%w: auth session %s requires a device ID", ErrInvalid, s.ID)
	}
	return nil
}
