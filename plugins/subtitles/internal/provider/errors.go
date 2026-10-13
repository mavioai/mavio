package provider

import (
	"errors"
	"fmt"
	"net"
	"time"
)

// Kind is why a provider failed, as the exception classes Bazarr throttles
// providers by.
type Kind int

// Kinds of failures.
const (
	// Other is any other failure, throttled 10 minutes.
	Other Kind = iota
	TooManyRequests
	DownloadLimitExceeded
	ServiceUnavailable
	APIThrottled
	ParseResponseError
	IPAddressBlocked
	Timeout
	ConfigurationError
	PermissionError
	ProxyError
	AuthenticationError
	SearchLimitReached
	ForbiddenError
	// ProviderError is a failure the provider reports.
	ProviderError
	// BadSubtitle is a subtitle that cannot be used, to be blocklisted
	// rather than throttled for: Bazarr's MustGetBlacklisted.
	BadSubtitle
)

var kindNames = [...]string{
	"Other", "TooManyRequests", "DownloadLimitExceeded", "ServiceUnavailable", "APIThrottled", "ParseResponseError",
	"IPAddressBlocked", "Timeout", "ConfigurationError", "PermissionError", "ProxyError", "AuthenticationError",
	"SearchLimitReached", "ForbiddenError", "ProviderError", "BadSubtitle",
}

// String returns the kind's name, as Bazarr's exception class.
func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// Error is a provider's failure of a kind.
type Error struct {
	Kind Kind
	Err  error
}

func (e *Error) Error() string { return e.Kind.String() + ": " + e.Err.Error() }

func (e *Error) Unwrap() error { return e.Err }

// Errorf returns a failure of a kind.
func Errorf(kind Kind, format string, args ...any) error {
	return &Error{Kind: kind, Err: fmt.Errorf(format, args...)}
}

// KindOf returns the kind of a failure: that of the Error it wraps,
// Timeout for network timeouts, else Other.
func KindOf(err error) Kind {
	if e, ok := errors.AsType[*Error](err); ok {
		return e.Kind
	}
	if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
		return Timeout
	}
	return Other
}

// Throttle returns how long Bazarr throttles a provider for a failure,
// with the provider's overrides.
func Throttle(def *Definition, kind Kind, now time.Time) time.Duration {
	if f, ok := def.Throttle[kind]; ok {
		return f(now)
	}
	if d, ok := defaultThrottle[kind]; ok {
		return d
	}
	return 10 * time.Minute
}

// defaultThrottle is Bazarr's provider_throttle_map()["default"].
var defaultThrottle = map[Kind]time.Duration{
	TooManyRequests:       time.Hour,
	DownloadLimitExceeded: 3 * time.Hour,
	ServiceUnavailable:    20 * time.Minute,
	APIThrottled:          10 * time.Minute,
	ParseResponseError:    6 * time.Hour,
	Timeout:               time.Hour,
	ConfigurationError:    12 * time.Hour,
	PermissionError:       12 * time.Hour,
	ProxyError:            time.Hour,
	AuthenticationError:   12 * time.Hour,
}

// Counted reports whether failures of a kind throttle a provider only
// after five of them within two minutes, as Bazarr's
// VALID_COUNT_EXCEPTIONS.
func Counted(k Kind) bool {
	switch k {
	case TooManyRequests, ServiceUnavailable, APIThrottled, Timeout:
		return true
	}
	return false
}

// Fixed returns a throttling duration that does not depend on the time.
func Fixed(d time.Duration) func(time.Time) time.Duration {
	return func(time.Time) time.Duration { return d }
}

// UntilMidnight returns the time until the next midnight in a time zone,
// plus extra: when sites in the zone reset their daily limits.
func UntilMidnight(zone string, extra time.Duration) func(time.Time) time.Duration {
	return func(now time.Time) time.Duration {
		loc, err := time.LoadLocation(zone)
		if err != nil {
			loc = time.UTC
		}
		t := now.In(loc)
		midnight := time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, loc)
		return midnight.Sub(t) + extra
	}
}
