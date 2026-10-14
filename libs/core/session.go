package core

import (
	"fmt"
	"time"
)

// AuthSession is a signed-in client: one device of a user, bound to the
// device's key. The server issues it short-lived access tokens, which the
// device renews by signing a nonce with that key; no token is stored.
type AuthSession struct {
	ID     ID
	UserID ID
	// DeviceKey is the device's Ed25519 public key.
	DeviceKey []byte
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
	case len(s.DeviceKey) != 32:
		return fmt.Errorf("%w: auth session %s requires an Ed25519 device key", ErrInvalid, s.ID)
	case s.DeviceID == "":
		return fmt.Errorf("%w: auth session %s requires a device ID", ErrInvalid, s.ID)
	}
	return nil
}
