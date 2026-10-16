package requests

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// fakeRatings rates every title it is asked of with its source, telling
// the name, year and IDs it was asked with; or fails, or answers
// unimplemented.
type fakeRatings struct {
	pluginv1connect.UnimplementedRatingsProviderServiceHandler
	source pluginv1.RatingSource
	fail   bool

	mu    sync.Mutex
	calls int
}

func (f *fakeRatings) GetRatings(_ context.Context, req *pluginv1.GetRatingsRequest) (*pluginv1.GetRatingsResponse, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	switch {
	case f.fail:
		return nil, errors.New("down")
	case f.source == pluginv1.RatingSource_RATING_SOURCE_UNSPECIFIED:
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("no ratings"))
	}
	return pluginv1.GetRatingsResponse_builder{Ratings: []*pluginv1.Rating{pluginv1.Rating_builder{
		Source: f.source.Enum(), Score: proto.Float64(7),
		Url: proto.String(fmt.Sprintf("%s %s %d %s %s", req.GetMediaKind(), req.GetName(), req.GetYear(), req.GetExternalIds()["tmdb"], req.GetExternalIds()["imdb"])),
	}.Build()}}.Build(), nil
}

func TestRatings(t *testing.T) {
	e := newEnv(t)
	imdb := &fakeRatings{source: pluginv1.RatingSource_RATING_SOURCE_IMDB}
	providers := []RatingsProvider{
		{ID: "none", Client: &fakeRatings{}},
		{ID: "imdb", Client: imdb},
	}
	e.svc.RatingsProviders = func() []RatingsProvider { return providers }

	got, err := e.svc.Ratings(e.ctx, e.admin, core.KindMovie, 1)
	if err != nil {
		t.Fatal(err)
	}
	var urls []string
	for _, r := range got {
		urls = append(urls, r.GetSource().String()+" "+r.GetUrl())
	}
	want := []string{"RATING_SOURCE_IMDB MEDIA_KIND_MOVIE Family Film 2001 1 tt1"}
	if !slices.Equal(urls, want) {
		t.Errorf("Ratings() = %q, want = %q", urls, want)
	}
	if e.meta.last.Language != "en" {
		t.Errorf("looked up in %q, want = en", e.meta.last.Language)
	}
	// Ratings are kept.
	if _, err := e.svc.Ratings(e.ctx, e.admin, core.KindMovie, 1); err != nil || imdb.calls != 1 {
		t.Errorf("Ratings() again asked %d times, %v, want = 1", imdb.calls, err)
	}

	g := e.user("g", func(u *core.User) { u.Access.MaxParentalRating = score(t, "G") })
	if _, err := e.svc.Ratings(e.ctx, g, core.KindMovie, 2); !errors.Is(err, ErrRating) {
		t.Errorf("Ratings() above the limit: error = %v, want ErrRating", err)
	}
	if _, err := e.svc.Ratings(e.ctx, e.admin, core.KindMovie, 404); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Ratings() of an unknown title: error = %v, want ErrNotFound", err)
	}

	// A provider failing is left out, and the ratings asked again.
	providers = append(providers, RatingsProvider{ID: "down", Client: &fakeRatings{fail: true}})
	for range 2 {
		got, err := e.svc.Ratings(e.ctx, e.admin, core.KindSeries, 9)
		if err != nil || len(got) != 1 {
			t.Fatalf("Ratings() with a provider failing = %v, %v, want one rating", got, err)
		}
	}
	if imdb.calls != 3 {
		t.Errorf("asked %d times, want = 3", imdb.calls)
	}
}
