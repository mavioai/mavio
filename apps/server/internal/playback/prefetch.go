package playback

import (
	"context"
	"errors"
	"slices"
	"sort"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/keyframes"
)

// Fetching ahead on storage that keeps what it reads, the block cache of
// rclone remotes: when a playback starts, and when a client asks for an
// HLS segment, the source's bytes for the next prefetchAhead of media are
// fetched before ffmpeg or the client reads them. A seek of HLS is a
// request for a segment.
const (
	// prefetchAhead is how much media is fetched ahead; maxPrefetch bounds
	// the bytes.
	prefetchAhead = 30 * time.Second
	maxPrefetch   = 256 << 20
)

// sourcePrefetch fetches a playback's source ahead of its reads. Its open
// file also marks where the playback is, so that the cache evicts what
// lies behind first.
type sourcePrefetch struct {
	f        core.File
	pf       core.Prefetcher
	st       core.Storage
	size     int64
	duration time.Duration
	// keyframes are the source's keyframes and offsets their file
	// offsets, from its container's index; nil when it gives none, and
	// offsets are estimated from the duration.
	keyframes []time.Duration
	offsets   []int64
}

// startPrefetch opens the source of a playback on storage that keeps what
// it reads, in the background, and fetches ahead from where it starts.
func (m *Manager) startPrefetch(ctx context.Context, p *Playback) {
	ms := p.Source()
	if p.root == "" || ms.Disc != "" || !core.IsRemotePath(ms.Path) {
		return
	}
	ctx = context.WithoutCancel(ctx)
	go func() {
		s, err := m.openPrefetch(ctx, p)
		if err != nil {
			m.log.DebugContext(ctx, "prefetch source", "playback", p.ID, "err", err)
			return
		}
		if s == nil {
			return
		}
		p.mu.Lock()
		ended := p.ended
		if !ended {
			p.prefetch = s
		}
		p.mu.Unlock()
		if ended {
			s.close()
			return
		}
		s.ahead(p.StartPosition)
	}()
}

// openPrefetch opens a playback's source and reads its keyframes' offsets;
// nil when its storage keeps nothing.
func (m *Manager) openPrefetch(ctx context.Context, p *Playback) (*sourcePrefetch, error) {
	ms := p.Source()
	name, ok := core.StorageName(p.root, ms.Path)
	if !ok {
		return nil, errors.New("source outside its library")
	}
	st, err := m.cfg.Storages.Open(ctx, p.root)
	if err != nil {
		return nil, err
	}
	// The index reads are few and small, and most hit what the probe
	// left in the cache.
	f, err := st.Open(core.WithReadIntent(ctx, core.ReadIntent{Kind: core.ReadBackground}), name)
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	pf, ok := f.(core.Prefetcher)
	if !ok {
		_ = f.Close()
		_ = st.Close()
		return nil, nil
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		_ = st.Close()
		return nil, err
	}
	s := &sourcePrefetch{f: f, pf: pf, st: st, size: info.Size(), duration: ms.Duration}
	if d, err := keyframes.FromIndex(f, s.size); err == nil && d.Offsets != nil && slices.IsSorted(d.Keyframes) && slices.IsSorted(d.Offsets) {
		s.keyframes, s.offsets = d.Keyframes, d.Offsets
	}
	return s, nil
}

// span returns the bytes [start, end) holding the media from from to to:
// from the keyframe at or before from to the one after to, or estimated
// from the size and duration; at most maxPrefetch bytes.
func (s *sourcePrefetch) span(from, to time.Duration) (start, end int64) {
	switch {
	case s.offsets != nil:
		if i := sort.Search(len(s.keyframes), func(i int) bool { return s.keyframes[i] > from }); i > 0 {
			start = s.offsets[i-1]
		}
		end = s.size
		if j := sort.Search(len(s.keyframes), func(j int) bool { return s.keyframes[j] > to }); j < len(s.offsets) {
			end = s.offsets[j]
		}
	case s.duration > 0:
		rate := float64(s.size) / s.duration.Seconds()
		start = int64(rate*from.Seconds()) - resumeLead
		end = int64(rate*to.Seconds()) + resumeLead
	default:
		return 0, 0
	}
	start = min(max(start, 0), s.size)
	return start, max(min(end, start+maxPrefetch, s.size), start)
}

// ahead fetches the media from at on, and marks at as where the playback
// is.
func (s *sourcePrefetch) ahead(at time.Duration) {
	s.pf.Prefetch(s.span(at, at+prefetchAhead))
}

// follow marks at as where the playback is, fetching nothing.
func (s *sourcePrefetch) follow(at time.Duration) {
	start, _ := s.span(at, at)
	s.pf.Prefetch(start, start)
}

func (s *sourcePrefetch) close() {
	_ = s.f.Close()
	_ = s.st.Close()
}

// prefetcher returns the playback's prefetch, nil when it has none.
func (p *Playback) prefetcher() *sourcePrefetch {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.prefetch
}
