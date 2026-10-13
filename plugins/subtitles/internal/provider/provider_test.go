package provider

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestThrottle(t *testing.T) {
	def := &Definition{Throttle: map[Kind]func(time.Time) time.Duration{TooManyRequests: Fixed(time.Minute)}}
	now := time.Date(2026, 10, 13, 22, 30, 0, 0, time.UTC)
	for _, c := range []struct {
		kind Kind
		want time.Duration
	}{
		{TooManyRequests, time.Minute},
		{DownloadLimitExceeded, 3 * time.Hour},
		{ServiceUnavailable, 20 * time.Minute},
		{AuthenticationError, 12 * time.Hour},
		{Other, 10 * time.Minute},
		{ProviderError, 10 * time.Minute},
	} {
		if got := Throttle(def, c.kind, now); got != c.want {
			t.Errorf("Throttle(%v) = %v, want = %v", c.kind, got, c.want)
		}
	}
}

func TestUntilMidnight(t *testing.T) {
	// 22:30 UTC is 00:30 in Prague's summer time, 23:30 in Lisbon's.
	now := time.Date(2026, 7, 13, 22, 30, 0, 0, time.UTC)
	for _, c := range []struct {
		zone  string
		extra time.Duration
		want  time.Duration
	}{
		{"Europe/Prague", 0, 23*time.Hour + 30*time.Minute},
		{"Europe/Lisbon", time.Hour, 90 * time.Minute},
		{"GMT", 15 * time.Minute, 105 * time.Minute},
	} {
		if got := UntilMidnight(c.zone, c.extra)(now); got != c.want {
			t.Errorf("UntilMidnight(%s) = %v, want = %v", c.zone, got, c.want)
		}
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestKindOf(t *testing.T) {
	for _, c := range []struct {
		err  error
		want Kind
	}{
		{Errorf(DownloadLimitExceeded, "limit"), DownloadLimitExceeded},
		{fmt.Errorf("search: %w", Errorf(ConfigurationError, "bad")), ConfigurationError},
		{fmt.Errorf("get: %w", timeoutError{}), Timeout},
		{errors.New("other"), Other},
	} {
		if got := KindOf(c.err); got != c.want {
			t.Errorf("KindOf(%v) = %v, want = %v", c.err, got, c.want)
		}
	}
	if got := Counted(Timeout); !got {
		t.Errorf("Counted(Timeout) = false, want = true")
	}
}

func TestRetry(t *testing.T) {
	for _, c := range []struct {
		err   error
		tries int
	}{
		{nil, 1},
		{Errorf(ProviderError, "x"), 3},
		{errors.New("x"), 3},
		{Errorf(TooManyRequests, "x"), 1},
		{Errorf(AuthenticationError, "x"), 1},
		{Errorf(ServiceUnavailable, "x"), 1},
	} {
		n := 0
		_, err := Retry(t.Context(), 3, 0, func() (int, error) {
			n++
			return n, c.err
		})
		if n != c.tries || !errors.Is(err, c.err) {
			t.Errorf("Retry(%v): tries = %d, err = %v, want = %d, %v", c.err, n, err, c.tries, c.err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Retry(ctx, 3, time.Hour, func() (int, error) { return 0, errors.New("x") }); !errors.Is(err, context.Canceled) {
		t.Errorf("Retry canceled: err = %v, want = %v", err, context.Canceled)
	}
}

func TestLanguageCodes(t *testing.T) {
	cases := []struct{ tag, alpha2, alpha3, alpha3B string }{
		{"fr", "fr", "fra", "fre"},
		{"pt-BR", "pt", "por", "por"},
		{"zh-Hant", "zh", "zho", "chi"},
		{"de", "de", "deu", "ger"},
	}
	for _, c := range cases {
		if got := Alpha2(c.tag); got != c.alpha2 {
			t.Errorf("Alpha2(%q) got = %q, want = %q", c.tag, got, c.alpha2)
		}
		if got := Alpha3(c.tag); got != c.alpha3 {
			t.Errorf("Alpha3(%q) got = %q, want = %q", c.tag, got, c.alpha3)
		}
		if got := Alpha3B(c.tag); got != c.alpha3B {
			t.Errorf("Alpha3B(%q) got = %q, want = %q", c.tag, got, c.alpha3B)
		}
	}
	for code, want := range map[string]string{"fre": "fr", "fra": "fr", "fr": "fr", "ger": "de", "chi": "zh", "xx1": ""} {
		if got := FromCode(code); got != want {
			t.Errorf("FromCode(%q) got = %q, want = %q", code, got, want)
		}
	}
	for name, want := range map[string]string{"French": "fr", "english": "en", "Serbian": "sr", "Klingonish": ""} {
		if got := FromName(name); got != want {
			t.Errorf("FromName(%q) got = %q, want = %q", name, got, want)
		}
	}
}
