package core

import "time"

// Play state thresholds, as Jellyfin's defaults.
const (
	// MinResumePercent: positions in the first 5% of the runtime are not
	// kept.
	MinResumePercent = 5
	// MaxResumePercent: positions past 90% of the runtime, or within a
	// second of the end, mark the item played.
	MaxResumePercent = 90
	// MinResumeRuntime: items shorter than this are marked played once
	// past MinResumePercent instead of keeping a position.
	MinResumeRuntime = 5 * time.Minute
	// MinAudiobookResume and MaxAudiobookRemaining replace the percentages
	// for audiobooks: positions in the first five minutes are not kept,
	// and positions in the last five mark the book played.
	MinAudiobookResume    = 5 * time.Minute
	MaxAudiobookRemaining = 5 * time.Minute
)

// SupportsPlayed reports whether items of the kind can be marked played.
func (k ItemKind) SupportsPlayed() bool {
	switch k {
	case KindMovie, KindEpisode, KindVideo, KindMusicVideo, KindTrack, KindAudioBook, KindBook:
		return true
	}
	return false
}

// SupportsResume reports whether playback of items of the kind resumes
// where it stopped.
func (k ItemKind) SupportsResume() bool {
	switch k {
	case KindMovie, KindEpisode, KindVideo, KindMusicVideo, KindAudioBook, KindBook:
		return true
	}
	return false
}

// RecordPosition updates the state for a playback of item that reached
// pos: it keeps pos as the resume position, or clears it near the start,
// or marks the item played near the end. It reports whether the item was
// played to completion; counting the play is up to the caller, once per
// playback.
func (d *UserData) RecordPosition(item *Item, pos time.Duration, now time.Time) (completed bool) {
	runtime := item.Runtime
	switch {
	case runtime <= 0:
		// Without a runtime, any playback counts as complete.
		completed, pos = true, 0
	case pos <= 0:
	case item.Kind == KindAudioBook:
		switch {
		case pos < MinAudiobookResume:
			pos = 0
		case runtime-pos < MaxAudiobookRemaining:
			completed, pos = true, 0
		}
	case item.Kind == KindBook:
	default:
		switch percent := float64(pos) / float64(runtime) * 100; {
		case percent < MinResumePercent:
			pos = 0
		case percent > MaxResumePercent || pos >= runtime-time.Second:
			completed, pos = true, 0
		case runtime < MinResumeRuntime:
			completed, pos = true, 0
		}
	}
	if completed {
		d.Played = true
	}
	if !item.Kind.SupportsPlayed() {
		d.Played, completed = false, false
	}
	if !item.Kind.SupportsResume() {
		pos = 0
	}
	d.Position = pos
	if completed || pos > 0 {
		d.LastPlayedAt = &now
	}
	return completed
}
