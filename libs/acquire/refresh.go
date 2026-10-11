package acquire

import "time"

// StatusAt returns the status of a movie at now from its release dates,
// as SkyHookProxy.MapMovie sets it: announced until it is in cinemas,
// released from its digital or physical release, or 90 days after it is
// in cinemas when it has neither.
func (a *Availability) StatusAt(now time.Time) MovieStatus {
	status := StatusAnnounced
	if !a.InCinemas.IsZero() && now.After(a.InCinemas) {
		status = StatusInCinemas
		if a.Physical.IsZero() && a.Digital.IsZero() && now.After(a.InCinemas.AddDate(0, 0, 90)) {
			status = StatusReleased
		}
	}
	if !a.Physical.IsZero() && !now.Before(a.Physical) || !a.Digital.IsZero() && !now.Before(a.Digital) {
		status = StatusReleased
	}
	return status
}

// physicalRelease returns the physical release of a movie, or 90 days
// after it is in cinemas without one, as Radarr's
// MovieMetadata.PhysicalReleaseDate; zero when neither is known, which
// Radarr takes as the end of time.
func (a *Availability) physicalRelease() time.Time {
	switch {
	case !a.Physical.IsZero():
		return a.Physical
	case !a.InCinemas.IsZero():
		return a.InCinemas.AddDate(0, 0, 90)
	}
	return time.Time{}
}

// ShouldRefreshMovie reports whether a movie's metadata, last refreshed
// at lastRefresh (zero for never), is to be refreshed at now, as Radarr's
// ShouldRefreshMovie: when it is older than 180 days, else when it is at
// least 12 hours old and the movie is announced, in cinemas or physically
// released in the last 30 days.
func ShouldRefreshMovie(lastRefresh time.Time, a *Availability, now time.Time) bool {
	switch {
	case lastRefresh.Before(now.AddDate(0, 0, -180)):
		return true
	case !lastRefresh.Before(now.Add(-12 * time.Hour)):
		return false
	}
	switch a.StatusAt(now) {
	case StatusAnnounced, StatusInCinemas:
		return true
	case StatusReleased:
		// Without a known physical release, it is still to come.
		p := a.physicalRelease()
		return p.IsZero() || !p.Before(now.AddDate(0, 0, -30))
	}
	return false
}
