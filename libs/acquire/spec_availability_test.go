package acquire

import (
	"testing"
	"time"
)

// TestAvailabilityAvailable ports Radarr's MovieTests/MovieIsAvailableFixture,
// the tests of Movie.IsAvailable. Its past date is 2000-01-01 and its
// future one 2100-01-01; Radarr compares them with the current time,
// here testNow.
func TestAvailabilityAvailable(t *testing.T) {
	past := time.Date(2000, 1, 1, 21, 10, 42, 0, time.UTC)
	future := time.Date(2100, 1, 1, 21, 10, 42, 0, time.UTC)
	var none time.Time
	type dates struct{ cinema, physical, digital time.Time }
	tests := []struct {
		name    string
		dates   dates
		minimum MovieStatus
		want    bool
	}{
		// Minimum availability TBA.
		{"TBA none", dates{none, none, none}, StatusTBA, true},
		{"TBA cinema past", dates{past, none, none}, StatusTBA, true},
		{"TBA cinema future", dates{future, none, none}, StatusTBA, true},
		{"TBA physical past", dates{none, past, none}, StatusTBA, true},
		{"TBA physical future", dates{none, future, none}, StatusTBA, true},
		{"TBA digital past", dates{none, none, past}, StatusTBA, true},
		{"TBA digital future", dates{none, none, future}, StatusTBA, true},

		// Minimum availability Announced.
		{"Announced none", dates{none, none, none}, StatusAnnounced, true},
		{"Announced cinema past", dates{past, none, none}, StatusAnnounced, true},
		{"Announced cinema future", dates{future, none, none}, StatusAnnounced, true},
		{"Announced physical past", dates{none, past, none}, StatusAnnounced, true},
		{"Announced physical future", dates{none, future, none}, StatusAnnounced, true},
		{"Announced digital past", dates{none, none, past}, StatusAnnounced, true},
		{"Announced digital future", dates{none, none, future}, StatusAnnounced, true},

		// Minimum availability InCinemas: in cinemas in the past, the others
		// unknown or in the future.
		{"InCinemas cinema past", dates{past, none, none}, StatusInCinemas, true},
		{"InCinemas cinema past physical future", dates{past, future, none}, StatusInCinemas, true},
		{"InCinemas cinema past physical future digital future", dates{past, future, future}, StatusInCinemas, true},
		{"InCinemas cinema past digital future", dates{past, none, future}, StatusInCinemas, true},
		// In cinemas in the future, the others unknown or in the future.
		{"InCinemas cinema future", dates{future, none, none}, StatusInCinemas, false},
		{"InCinemas cinema future physical future", dates{future, future, none}, StatusInCinemas, false},
		{"InCinemas cinema future physical future digital future", dates{future, future, future}, StatusInCinemas, false},
		{"InCinemas cinema future digital future", dates{future, none, future}, StatusInCinemas, false},
		// In cinemas unknown, physical or digital past.
		{"InCinemas physical past", dates{none, past, none}, StatusInCinemas, true},
		{"InCinemas physical past digital past", dates{none, past, past}, StatusInCinemas, true},
		{"InCinemas digital past", dates{none, none, past}, StatusInCinemas, true},
		// The same with physical or digital in the future.
		{"InCinemas physical future", dates{none, future, none}, StatusInCinemas, false},
		{"InCinemas physical future digital future", dates{none, future, future}, StatusInCinemas, false},
		{"InCinemas digital future", dates{none, none, future}, StatusInCinemas, false},
		// No dates.
		{"InCinemas none", dates{none, none, none}, StatusInCinemas, false},

		// Minimum availability Released.
		{"Released none", dates{none, none, none}, StatusReleased, false},
		{"Released cinema past", dates{past, none, none}, StatusReleased, true},
		{"Released cinema future", dates{future, none, none}, StatusReleased, false},
		{"Released physical past", dates{none, past, none}, StatusReleased, true},
		{"Released physical future", dates{none, future, none}, StatusReleased, false},
		{"Released digital past", dates{none, none, past}, StatusReleased, true},
		{"Released digital future", dates{none, none, future}, StatusReleased, false},
	}
	for _, tt := range tests {
		t.Run("should_have_correct_availability/"+tt.name, func(t *testing.T) {
			a := &Availability{Minimum: tt.minimum, InCinemas: tt.dates.cinema, Physical: tt.dates.physical, Digital: tt.dates.digital}
			if got := a.Available(testNow, 0); got != tt.want {
				t.Errorf("Available() = %v, want = %v", got, tt.want)
			}
		})
	}
}

// TestAvailabilityDelay ports the delay and 90-day tests of Radarr's
// MovieTests/MovieIsAvailableFixture.
func TestAvailabilityDelay(t *testing.T) {
	days := func(n int) time.Time { return testNow.AddDate(0, 0, n) }
	var none time.Time
	tests := []struct {
		name             string
		cinema, physical time.Time
		delay            int
		announced        bool
		want             bool
	}{
		{"positive_delay_should_effect_availability (no delay)", none, days(-5), 0, false, true},
		{"positive_delay_should_effect_availability (delay 6)", none, days(-5), 6, false, false},
		{"negative_delay_should_effect_availability (no delay)", none, days(5), 0, false, false},
		{"negative_delay_should_effect_availability (delay -6)", none, days(5), -6, false, true},
		{"minimum_availability_released_no_date_but_ninety_days_after_cinemas (cinema -91)", days(-91), none, 0, false, true},
		{"minimum_availability_released_no_date_but_ninety_days_after_cinemas (cinema -89)", days(-89), none, 0, false, false},
		{"minimum_availability_released_no_date_but_ninety_days_after_cinemas (cinema -89, physical -40)", days(-89), days(-40), 0, false, true},
		{"minimum_availability_released_no_date_but_ninety_days_after_cinemas (cinema -91, physical +40)", days(-91), days(40), 0, false, false},
		// Not in the fixture: the delay does not apply to TBA and
		// Announced, which are available at once.
		{"announced ignores delay", none, none, 1000, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Availability{Minimum: StatusReleased, InCinemas: tt.cinema, Physical: tt.physical}
			if tt.announced {
				a.Minimum = StatusAnnounced
			}
			if got := a.Available(testNow, tt.delay); got != tt.want {
				t.Errorf("Available(%d) = %v, want = %v", tt.delay, got, tt.want)
			}
		})
	}
}
