package library

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

type remoteShelf struct {
	sources []RemoteSource
	fail    bool
}

func (*remoteShelf) Name() string { return "shelf" }

func (s *remoteShelf) MediaSources(context.Context, core.Item) ([]RemoteSource, error) {
	if s.fail {
		return nil, errors.New("down")
	}
	return s.sources, nil
}

// urlProber probes URLs ending in ".mkv" and counts its probes.
type urlProber struct{ probes int }

func (p *urlProber) ProbeURL(_ context.Context, u string, audio bool) (core.MediaSource, error) {
	p.probes++
	if len(u) < 4 || u[len(u)-4:] != ".mkv" {
		return core.MediaSource{}, errors.New("unreadable")
	}
	kind := core.StreamVideo
	if audio {
		kind = core.StreamAudio
	}
	return core.MediaSource{Container: "mkv", Duration: time.Hour, Streams: []core.MediaStream{{Kind: kind, Codec: "h264"}}}, nil
}

func TestRemoteSources(t *testing.T) {
	ctx := t.Context()
	now := time.Now()
	shelf := &remoteShelf{sources: []RemoteSource{
		{ID: "a", Name: "4K", URL: "https://media.test/a.mkv"},
		{ID: "b", URL: "file:///etc/passwd"},
		{ID: "c", URL: "https://media.test/c.txt"},
	}}
	prober := &urlProber{}
	r := &RemoteSources{Source: func() []MediaSourceProvider { return []MediaSourceProvider{&remoteShelf{fail: true}, shelf} }, Prober: prober}
	r.now = func() time.Time { return now }
	it := core.Item{ID: core.NewID(), Kind: core.KindMovie}

	got := r.For(ctx, it)
	if len(got) != 1 {
		t.Fatalf("For() = %+v, want one source", got)
	}
	ms := got[0]
	if ms.ItemID != it.ID || ms.Path != "https://media.test/a.mkv" || ms.Name != "4K" || ms.Container != "mkv" || ms.Streams[0].Kind != core.StreamVideo {
		t.Errorf("source = %+v", ms)
	}
	if ms.ID != remoteSourceID("shelf", it.ID, "a") || ms.ID == remoteSourceID("shelf", it.ID, "b") {
		t.Errorf("ID = %s, not derived from the provider, item and source", ms.ID)
	}
	// Two URLs were probed; the file URL was not.
	if prober.probes != 2 {
		t.Errorf("probes = %d, want 2", prober.probes)
	}

	// Probes, failed ones included, are reused for a while.
	r.For(ctx, it)
	if prober.probes != 2 {
		t.Errorf("probes after a second look = %d, want 2", prober.probes)
	}
	now = now.Add(RemoteProbeTTL / 12)
	r.For(ctx, it)
	if prober.probes != 3 {
		t.Errorf("probes once failures expired = %d, want 3", prober.probes)
	}
	now = now.Add(RemoteProbeTTL)
	r.For(ctx, it)
	if prober.probes != 5 {
		t.Errorf("probes once all expired = %d, want 5", prober.probes)
	}
}
