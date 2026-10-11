package acquire

import (
	"context"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// rkCountingPorts counts the calls to Pending of the ports it wraps.
type rkCountingPorts struct {
	*fakePorts
	pendingCalls int
}

func (p *rkCountingPorts) Pending(ctx context.Context, id core.ID) ([]Release, error) {
	p.pendingCalls++
	return p.fakePorts.Pending(ctx, id)
}

// TestPendingSpec ports Radarr's RssSync/PendingSpecificationFixture. Radarr
// reads the whole pending queue and matches the movie; the port reads the
// pending releases of the target, so a pending release of another movie
// is one kept under another target ID.
func TestPendingSpec(t *testing.T) {
	tests := []struct {
		name    string
		source  Source
		pending map[core.ID][]Release
		want    bool
		// wantCalls is how often the pending releases are read.
		wantCalls int
	}{
		{
			name:      "should_return_true_when_pending_queue_is_empty",
			source:    SourceReleasePush,
			want:      true,
			wantCalls: 1,
		},
		{
			name:      "should_return_true_when_not_pushed_release",
			source:    SourceRSS,
			want:      true,
			wantCalls: 0,
		},
		{
			name:      "should_return_true_when_movie_is_not_pending",
			source:    SourceReleasePush,
			pending:   map[core.ID][]Release{tid(2): {{Title: "Other.Movie.2024.720p.HDTV-Group"}}},
			want:      true,
			wantCalls: 1,
		},
		{
			name:      "should_return_false_when_movie_is_pending",
			source:    SourceReleasePush,
			pending:   map[core.ID][]Release{tid(1): {{Title: "Movie.Title.2024.720p.HDTV-Group"}}},
			want:      false,
			wantCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, fake := newTestEnv()
			fake.pending = tt.pending
			ports := &rkCountingPorts{fakePorts: fake}
			e.Ports = ports
			c := testMovie(testProfile(release.DVD, release.DVD), release.DVD)
			c.Languages = []release.Language{release.Spanish}
			c.Source = tt.source
			r := check(t, Pending{}, e, c, nil)
			if got := r == nil; got != tt.want {
				t.Errorf("accepted = %v, want = %v", got, tt.want)
			}
			if r != nil && r.Reason != ReasonMinimumAgeDelayPushed {
				t.Errorf("reason = %v, want = %v", r.Reason, ReasonMinimumAgeDelayPushed)
			}
			if ports.pendingCalls != tt.wantCalls {
				t.Errorf("Pending calls = %d, want = %d", ports.pendingCalls, tt.wantCalls)
			}
		})
	}
}
