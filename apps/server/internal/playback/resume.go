package playback

import (
	"context"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library/storage"
	"github.com/mavioai/mavio/libs/media/decision"
)

// Resuming, as khuaplayer does: the seek is folded into preparing the
// playback, playback begins on a keyframe when one is close before the
// resume position, and the media there is read ahead.
const (
	// snapWithin is how far before the resume position playback may
	// begin, at a keyframe, instead of decoding from the keyframe only to
	// drop the frames up to the position.
	snapWithin = 5 * time.Second
	// resumeAhead is how much media after the resume position is read
	// ahead, and resumeLead how much before it, the estimate of where it
	// lies being rough; maxResumeWarm bounds the bytes.
	resumeAhead   = 3 * time.Second
	resumeLead    = 1 << 20
	maxResumeWarm = 64 << 20
)

// startPosition returns where the playback begins for a client asking to
// begin at at: the start of the HLS segment playing then, which the
// encoder or the source begins with a keyframe, or for direct play the
// source's keyframe before it, when that is at most snapWithin before.
// Otherwise it is at.
func (p *Playback) startPosition(at time.Duration) time.Duration {
	if at <= 0 {
		return 0
	}
	var keyframe time.Duration
	switch {
	case p.stream != nil && len(p.layout.Segments) > 0:
		keyframe = p.layout.Start(p.layout.Index(at))
	case p.Method == decision.DirectPlay && len(p.Source().Keyframes) > 0:
		kf := p.Source().Keyframes
		i, found := slices.BinarySearch(kf, at)
		if found || i == 0 {
			return at
		}
		keyframe = kf[i-1]
	default:
		return at
	}
	if at-keyframe <= snapWithin {
		return keyframe
	}
	return at
}

// prepareStart begins the playback where it starts: an HLS stream starts
// ffmpeg there at once, and the source's container header and index and
// the media there are read ahead in the background.
func (m *Manager) prepareStart(ctx context.Context, p *Playback) {
	if p.stream != nil {
		if err := p.stream.Prepare(context.WithoutCancel(ctx), p.StartPosition); err != nil {
			m.log.WarnContext(ctx, "start transcode", "playback", p.ID, "err", err)
		}
	}
	ms := p.Source()
	if ms.Path == "" || ms.Disc != "" {
		return
	}
	go warmResume(*ms, p.StartPosition)
}

// warmResume reads ahead the header and index of a source, and the media
// around at, estimated from its size and duration.
func warmResume(ms core.MediaSource, at time.Duration) {
	_ = storage.PrefetchHeadTail(ms.Path, 0, 0)
	if at <= 0 || ms.Duration <= 0 || ms.Size <= 0 {
		return
	}
	rate := float64(ms.Size) / ms.Duration.Seconds()
	offset := int64(rate*at.Seconds()) - resumeLead
	n := min(int64(rate*resumeAhead.Seconds())+resumeLead, maxResumeWarm)
	_ = storage.PrefetchRange(ms.Path, offset, n)
}

// keepAwake beats the volumes the playbacks in progress read from, so
// that a paused playback resumes without waiting for a disk to spin up
// or a share to reconnect.
func (m *Manager) keepAwake() {
	m.mu.Lock()
	paths := make([]string, 0, len(m.playbacks))
	for _, p := range m.playbacks {
		if ms := p.Source(); ms.Path != "" && ms.Disc == "" {
			paths = append(paths, ms.Path)
		}
	}
	m.mu.Unlock()
	m.cfg.Keeper.Beat(paths...)
}
