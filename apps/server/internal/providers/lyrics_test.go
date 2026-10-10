package providers

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

type fakeLyrics struct{ got *pluginv1.SearchLyricsRequest }

func (f *fakeLyrics) SearchLyrics(_ context.Context, req *pluginv1.SearchLyricsRequest) (*pluginv1.SearchLyricsResponse, error) {
	f.got = req
	return pluginv1.SearchLyricsResponse_builder{Lyrics: []*pluginv1.RemoteLyrics{
		pluginv1.RemoteLyrics_builder{Id: proto.String("7"), Name: proto.String("One"), Duration: durationpb.New(time.Minute), Synced: proto.Bool(true)}.Build(),
		pluginv1.RemoteLyrics_builder{Name: proto.String("no ID")}.Build(),
	}}.Build(), nil
}

func (f *fakeLyrics) DownloadLyrics(_ context.Context, req *pluginv1.DownloadLyricsRequest) (*pluginv1.DownloadLyricsResponse, error) {
	return pluginv1.DownloadLyricsResponse_builder{Content: proto.String("[00:01.00]" + req.GetId()), Synced: proto.Bool(true)}.Build(), nil
}

func TestLyricsPlugin(t *testing.T) {
	f := &fakeLyrics{}
	p := &LyricsPlugin{ID: "lrclib", Client: f}
	found, err := p.SearchLyrics(t.Context(), library.LyricsQuery{
		Name: "One", Artists: []string{"Band"}, Duration: 2 * time.Minute, ExternalIDs: map[core.Provider]string{core.ProviderMusicBrainzTrack: "x"},
	})
	if err != nil || len(found) != 1 || found[0].ID != "7" || found[0].Duration != time.Minute || !found[0].Synced {
		t.Errorf("SearchLyrics() = %+v, %v", found, err)
	}
	if f.got.GetName() != "One" || f.got.GetDuration().AsDuration() != 2*time.Minute || f.got.GetExternalIds()["musicbrainz_track"] != "x" || f.got.HasAlbum() {
		t.Errorf("request = %v", f.got)
	}
	if content, synced, err := p.DownloadLyrics(t.Context(), "7"); err != nil || content != "[00:01.00]7" || !synced {
		t.Errorf("DownloadLyrics() = %q, %v, %v", content, synced, err)
	}
}
