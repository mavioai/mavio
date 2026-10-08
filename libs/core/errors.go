package core

import "errors"

// Sentinel errors returned by implementations of the ports in this package.
// Wrap them with context and test with errors.Is.
var (
	// ErrNotFound means the requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict means the operation conflicts with existing state, such as a
	// duplicate unique key or a stale version.
	ErrConflict = errors.New("conflict")
	// ErrInvalid means the input violates a domain rule.
	ErrInvalid = errors.New("invalid")
)
