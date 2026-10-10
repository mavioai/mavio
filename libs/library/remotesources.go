package library

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// MediaSourceProvider gives items further media sources read over HTTP,
// such as a media source provider plugin.
type MediaSourceProvider interface {
	Name() string
	MediaSources(ctx context.Context, it core.Item) ([]RemoteSource, error)
}

// RemoteSource is media read over HTTP.
type RemoteSource struct {
	// ID is stable among the item's sources from the provider.
	ID, Name string
	// URL is an absolute http(s) URL.
	URL string
}

// URLProber reads the facts of media at a URL, such as with ffprobe.
type URLProber interface {
	ProbeURL(ctx context.Context, url string, audio bool) (core.MediaSource, error)
}

// Limits of RemoteSources.
const (
	// RemoteProbeTTL is how long a probe of a URL is reused; failures are
	// remembered a twelfth of it.
	RemoteProbeTTL = time.Hour
	// RemoteProbeTimeout bounds a probe.
	RemoteProbeTimeout = 30 * time.Second
	// maxRemoteProbes bounds the probes remembered.
	maxRemoteProbes = 256
)

// RemoteSources finds the media sources the providers give items, probed
// and ready to play.
type RemoteSources struct {
	// Source gives the providers, in order.
	Source func() []MediaSourceProvider
	Prober URLProber
	Logger *slog.Logger

	mu     sync.Mutex
	probes map[string]remoteProbe
	now    func() time.Time
}

type remoteProbe struct {
	source core.MediaSource
	err    error
	at     time.Time
}

func (r *RemoteSources) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.New(slog.DiscardHandler)
}

func (r *RemoteSources) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

// For returns the sources the providers give an item, in their order,
// leaving out those that are not http(s) URLs or fail to probe. Each has
// an ID derived from the provider, the item and its ID among the
// provider's, its URL as Path, and the facts probed.
func (r *RemoteSources) For(ctx context.Context, it core.Item) []core.MediaSource {
	if r == nil || r.Source == nil || r.Prober == nil {
		return nil
	}
	audio := it.Kind == core.KindTrack || it.Kind == core.KindAudioBook
	var out []core.MediaSource
	for _, p := range r.Source() {
		sources, err := p.MediaSources(ctx, it)
		if err != nil {
			r.logger().WarnContext(ctx, "media source provider failed", "provider", p.Name(), "item", it.ID, "err", err)
			continue
		}
		for _, s := range sources {
			if u, err := url.Parse(s.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				r.logger().WarnContext(ctx, "media source provider gave a source that is not an http(s) URL",
					"provider", p.Name(), "item", it.ID, "source", s.ID)
				continue
			}
			ms, err := r.probe(ctx, s.URL, audio)
			if err != nil {
				r.logger().WarnContext(ctx, "probing a remote media source failed", "provider", p.Name(), "item", it.ID, "source", s.ID, "err", err)
				continue
			}
			ms.ID, ms.ItemID, ms.Path, ms.Name = remoteSourceID(p.Name(), it.ID, s.ID), it.ID, s.URL, s.Name
			out = append(out, ms)
		}
	}
	return out
}

// probe probes a URL, or reuses a recent probe of it.
func (r *RemoteSources) probe(ctx context.Context, u string, audio bool) (core.MediaSource, error) {
	key := fmt.Sprint(audio, " ", u)
	now := r.clock()
	r.mu.Lock()
	if p, ok := r.probes[key]; ok {
		ttl := RemoteProbeTTL
		if p.err != nil {
			ttl /= 12
		}
		if now.Sub(p.at) < ttl {
			r.mu.Unlock()
			return cloneSource(p.source), p.err
		}
	}
	r.mu.Unlock()
	pctx, cancel := context.WithTimeout(ctx, RemoteProbeTimeout)
	ms, err := r.Prober.ProbeURL(pctx, u, audio)
	cancel()
	if ctx.Err() != nil {
		return ms, ctx.Err()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.probes == nil {
		r.probes = map[string]remoteProbe{}
	}
	if len(r.probes) >= maxRemoteProbes {
		oldest := ""
		for k, p := range r.probes {
			if oldest == "" || p.at.Before(r.probes[oldest].at) {
				oldest = k
			}
		}
		delete(r.probes, oldest)
	}
	r.probes[key] = remoteProbe{source: cloneSource(ms), err: err, at: now}
	return ms, err
}

// cloneSource copies a source's streams and chapters, which callers may
// change.
func cloneSource(ms core.MediaSource) core.MediaSource {
	ms.Streams = append([]core.MediaStream(nil), ms.Streams...)
	ms.Chapters = append([]core.Chapter(nil), ms.Chapters...)
	return ms
}

// remoteSourceID derives the ID of a provider's source of an item, the
// same each time.
func remoteSourceID(provider string, item core.ID, id string) core.ID {
	sum := sha256.Sum256([]byte(provider + "\x00" + item.String() + "\x00" + id))
	var out core.ID
	copy(out[:], sum[:16])
	out[6] = out[6]&0x0f | 0x80 // version 8
	out[8] = out[8]&0x3f | 0x80 // RFC 9562 variant
	return out
}
