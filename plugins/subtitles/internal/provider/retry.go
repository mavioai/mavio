package provider

import (
	"context"
	"time"
)

// Retry calls f up to amount times, waiting between calls, as Bazarr's
// ProviderRetryMixin: failures that retrying cannot help (service
// unavailable, too many requests, download limits, configuration and
// authentication) are returned at once.
func Retry[T any](ctx context.Context, amount int, wait time.Duration, f func() (T, error)) (T, error) {
	for i := 1; ; i++ {
		out, err := f()
		if err == nil || i >= amount {
			return out, err
		}
		switch KindOf(err) {
		case ServiceUnavailable, TooManyRequests, DownloadLimitExceeded, ConfigurationError, AuthenticationError:
			return out, err
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return out, ctx.Err()
		case <-t.C:
		}
	}
}
